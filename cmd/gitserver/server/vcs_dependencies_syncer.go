package server

import (
	"context"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/internal/api"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// vcsPackagesSyncer implements the VCSSyncer interface for dependency repos
// of different types.
type vcsPackagesSyncer struct {
	logger log.Logger
	typ    string
	scheme string

	// placeholder is used to set GIT_AUTHOR_NAME for git commands that don't create
	// commits or tags. The name of this dependency should never be publicly visible,
	// so it can have any random value.
	placeholder reposource.VersionedPackage
	configDeps  []string
	source      packagesSource
	svc         dependenciesService
}

var _ VCSSyncer = &vcsPackagesSyncer{}

// packagesSource encapsulates the methods required to implement a source of
// package dependencies e.g. npm, go modules, jvm, python.
type packagesSource interface {
	// Download the given dependency's archive and unpack it into dir.
	Download(ctx context.Context, dir string, dep reposource.VersionedPackage) error

	ParseVersionedPackageFromNameAndVersion(name reposource.PackageName, version string) (reposource.VersionedPackage, error)
	// ParseVersionedPackageFromConfiguration parses a package and version from the "dependencies"
	// field from the site-admin interface.
	ParseVersionedPackageFromConfiguration(dep string) (reposource.VersionedPackage, error)
	// ParsePackageFromRepoName parses a Sourcegraph repository name of the package.
	ParsePackageFromRepoName(repoName api.RepoName) (reposource.Package, error)
}

type packagesDownloadSource interface {
	// GetPackage sends a request to the package host to get metadata about this package, like the description.
	GetPackage(ctx context.Context, name reposource.PackageName) (reposource.Package, error)
}

// dependenciesService captures the methods we use of the codeintel/dependencies.Service,
// used to make testing easier.
type dependenciesService interface {
	ListDependencyRepos(context.Context, dependencies.ListDependencyReposOpts) ([]dependencies.Repo, error)
}

func (s *vcsPackagesSyncer) IsCloneable(ctx context.Context, repoUrl *vcs.URL) error {
	return nil
}

func (s *vcsPackagesSyncer) Type() string {
	return s.typ
}

func (s *vcsPackagesSyncer) RemoteShowCommand(ctx context.Context, remoteURL *vcs.URL) (cmd *exec.Cmd, err error) {
	return exec.CommandContext(ctx, "git", "remote", "show", "./"), nil
}

func (s *vcsPackagesSyncer) CloneCommand(ctx context.Context, remoteURL *vcs.URL, bareGitDirectory string) (*exec.Cmd, error) {
	err := os.MkdirAll(bareGitDirectory, 0755)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "git", "--bare", "init")
	if _, err := runCommandInDirectory(ctx, cmd, bareGitDirectory, s.placeholder); err != nil {
		return nil, err
	}

	// The Fetch method is responsible for cleaning up temporary directories.
	if err := s.Fetch(ctx, remoteURL, GitDir(bareGitDirectory)); err != nil {
		return nil, errors.Wrapf(err, "failed to fetch repo for %s", remoteURL)
	}

	// no-op command to satisfy VCSSyncer interface, see docstring for more details.
	return exec.CommandContext(ctx, "git", "--version"), nil
}

func (s *vcsPackagesSyncer) Fetch(ctx context.Context, remoteURL *vcs.URL, dir GitDir) (err error) {
	var dep reposource.Package
	dep, err = s.source.ParsePackageFromRepoName(api.RepoName(remoteURL.Path))
	if err != nil {
		return err
	}

	depName := dep.PackageSyntax()

	var versions []string
	versions, err = s.versions(ctx, depName)
	if err != nil {
		return err
	}

	var errs errors.MultiError
	cloneable := make([]reposource.VersionedPackage, 0, len(versions))
	for _, version := range versions {
		if d, err := s.source.ParseVersionedPackageFromNameAndVersion(depName, version); err != nil {
			errs = errors.Append(errs, err)
		} else {
			cloneable = append(cloneable, d)
		}
	}
	if errs != nil {
		return errs
	}

	// We sort in descending order, so that the latest version is in the first position.
	sort.SliceStable(cloneable, func(i, j int) bool {
		return cloneable[i].Less(cloneable[j])
	})

	// Create set of existing tags. We want to skip the download of a package if the
	// tag already exists.
	out, err := runCommandInDirectory(ctx, exec.CommandContext(ctx, "git", "tag"), string(dir), s.placeholder)
	if err != nil {
		return err
	}

	tags := map[string]struct{}{}
	for _, line := range strings.Split(out, "\n") {
		if len(line) == 0 {
			continue
		}
		tags[line] = struct{}{}
	}

	var cloned []reposource.VersionedPackage
	for _, dependency := range cloneable {
		if _, tagExists := tags[dependency.GitTagFromVersion()]; tagExists {
			continue
		}
		if err := s.gitPushDependencyTag(ctx, string(dir), dependency); err != nil {
			errs = errors.Append(errs, errors.Wrapf(err, "error pushing dependency %q", dependency))
		} else {
			cloned = append(cloned, dependency)
		}
	}

	// Set the latest version as the default branch, if there was a successful download.
	if len(cloned) > 0 {
		latest := cloned[0]
		cmd := exec.CommandContext(ctx, "git", "branch", "--force", "latest", latest.GitTagFromVersion())
		if _, err := runCommandInDirectory(ctx, cmd, string(dir), latest); err != nil {
			return errors.Append(errs, err)
		}
	}

	// Return error if at least one version failed to download.
	if errs != nil {
		return errs
	}

	// Delete tags for versions we no longer track if there were no errors so far.
	dependencyTags := make(map[string]struct{}, len(cloneable))
	for _, dependency := range cloneable {
		dependencyTags[dependency.GitTagFromVersion()] = struct{}{}
	}

	for tag := range tags {
		if _, isDependencyTag := dependencyTags[tag]; !isDependencyTag {
			cmd := exec.CommandContext(ctx, "git", "tag", "-d", tag)
			if _, err := runCommandInDirectory(ctx, cmd, string(dir), s.placeholder); err != nil {
				s.logger.Error("failed to delete git tag",
					log.Error(err),
					log.String("tag", tag),
				)
				continue
			}
		}
	}

	if len(cloneable) == 0 {
		cmd := exec.CommandContext(ctx, "git", "branch", "--force", "-D", "latest")
		// Best-effort branch deletion since we don't know if this branch has been created yet.
		_, _ = runCommandInDirectory(ctx, cmd, string(dir), s.placeholder)
	}

	return nil
}

// gitPushDependencyTag downloads the dependency dep and updates
// bareGitDirectory. If successful, bareGitDirectory will contain a new tag based
// on dep.
//
// gitPushDependencyTag is responsible for cleaning up temporary directories
// created in the process.
func (s *vcsPackagesSyncer) gitPushDependencyTag(ctx context.Context, bareGitDirectory string, dep reposource.VersionedPackage) error {
	workDir, err := os.MkdirTemp("", s.Type())
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	err = s.source.Download(ctx, workDir, dep)
	// We should not return err when dependency is not found
	if err != nil && errcode.IsNotFound(err) {
		s.logger.With(
			log.String("dependency", dep.VersionedPackageSyntax()),
			log.String("error", err.Error()),
		).Warn("Error during dependency download")
		return nil
	} else if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "git", "init")
	if _, err := runCommandInDirectory(ctx, cmd, workDir, dep); err != nil {
		return err
	}

	cmd = exec.CommandContext(ctx, "git", "add", ".")
	if _, err := runCommandInDirectory(ctx, cmd, workDir, dep); err != nil {
		return err
	}

	// Use --no-verify for security reasons. See https://github.com/sourcegraph/sourcegraph/pull/23399
	cmd = exec.CommandContext(ctx, "git", "commit", "--no-verify",
		"-m", dep.VersionedPackageSyntax(), "--date", stableGitCommitDate)
	if _, err := runCommandInDirectory(ctx, cmd, workDir, dep); err != nil {
		return err
	}

	cmd = exec.CommandContext(ctx, "git", "tag",
		"-m", dep.VersionedPackageSyntax(), dep.GitTagFromVersion())
	if _, err := runCommandInDirectory(ctx, cmd, workDir, dep); err != nil {
		return err
	}

	cmd = exec.CommandContext(ctx, "git", "remote", "add", "origin", bareGitDirectory)
	if _, err := runCommandInDirectory(ctx, cmd, workDir, dep); err != nil {
		return err
	}

	// Use --no-verify for security reasons. See https://github.com/sourcegraph/sourcegraph/pull/23399
	cmd = exec.CommandContext(ctx, "git", "push", "--no-verify", "--force", "origin", "--tags")
	if _, err := runCommandInDirectory(ctx, cmd, workDir, dep); err != nil {
		return err
	}

	return nil
}

func (s *vcsPackagesSyncer) versions(ctx context.Context, packageName reposource.PackageName) ([]string, error) {
	var versions []string
	for _, d := range s.configDeps {
		dep, err := s.source.ParseVersionedPackageFromConfiguration(d)
		if err != nil {
			s.logger.Warn("skipping malformed dependency", log.String("dep", d), log.Error(err))
			continue
		}

		if dep.PackageSyntax() == packageName {
			versions = append(versions, dep.PackageVersion())
		}
	}

	depRepos, err := s.svc.ListDependencyRepos(ctx, dependencies.ListDependencyReposOpts{
		Scheme:      s.scheme,
		Name:        packageName,
		NewestFirst: true,
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to list dependencies from db")
	}

	for _, depRepo := range depRepos {
		versions = append(versions, depRepo.Version)
	}

	return versions, nil
}

func runCommandInDirectory(ctx context.Context, cmd *exec.Cmd, workingDirectory string, dependency reposource.VersionedPackage) (string, error) {
	gitName := dependency.VersionedPackageSyntax() + " authors"
	gitEmail := "code-intel@sourcegraph.com"
	cmd.Dir = workingDirectory
	cmd.Env = append(cmd.Env, "EMAIL="+gitEmail)
	cmd.Env = append(cmd.Env, "GIT_AUTHOR_NAME="+gitName)
	cmd.Env = append(cmd.Env, "GIT_AUTHOR_EMAIL="+gitEmail)
	cmd.Env = append(cmd.Env, "GIT_AUTHOR_DATE="+stableGitCommitDate)
	cmd.Env = append(cmd.Env, "GIT_COMMITTER_NAME="+gitName)
	cmd.Env = append(cmd.Env, "GIT_COMMITTER_EMAIL="+gitEmail)
	cmd.Env = append(cmd.Env, "GIT_COMMITTER_DATE="+stableGitCommitDate)
	output, err := runWith(ctx, cmd, false, nil)
	if err != nil {
		return "", errors.Wrapf(err, "command %s failed with output %s", cmd.Args, string(output))
	}
	return string(output), nil
}

func isPotentiallyMaliciousFilepathInArchive(filepath, destinationDir string) (outputPath string, _ bool) {
	if strings.HasSuffix(filepath, "/") {
		// Skip directory entries. Directory entries must end
		// with a forward slash (even on Windows) according to
		// `file.Name` docstring.
		return "", true
	}

	if strings.HasPrefix(filepath, "/") {
		// Skip absolute paths. While they are extracted relative to `destination`,
		// they should be unimportant. Related issue https://github.com/golang/go/issues/48085#issuecomment-912659635
		return "", true
	}

	for _, dirEntry := range strings.Split(filepath, string(os.PathSeparator)) {
		if dirEntry == ".git" {
			// For security reasons, don't unzip files under any `.git/`
			// directory. See https://github.com/sourcegraph/security-issues/issues/163
			return "", true
		}
	}

	cleanedOutputPath := path.Join(destinationDir, filepath)
	if !strings.HasPrefix(cleanedOutputPath, destinationDir) {
		// For security reasons, skip file if it's not a child
		// of the target directory. See "Zip Slip Vulnerability".
		return "", true
	}

	return cleanedOutputPath, false
}
