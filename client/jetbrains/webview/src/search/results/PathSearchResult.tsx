import React from 'react'

import FileDocumentIcon from 'mdi-react/FileDocumentIcon'

import { formatRepositoryStarCount, LastSyncedIcon, SearchResultStar } from '@sourcegraph/search-ui'
import { PathMatch } from '@sourcegraph/shared/src/search/stream'

import { RepoName } from './RepoName'
import { SearchResultLayout } from './SearchResultLayout'
import { SelectableSearchResult } from './SelectableSearchResult'

interface Props {
    match: PathMatch
    selectedResult: null | string
    lastSyncedTime?: string
    selectResult: (id: string) => void
}

export const PathSearchResult: React.FunctionComponent<Props> = ({
    match,
    selectedResult,
    lastSyncedTime,
    selectResult,
}: Props) => {
    const formattedRepositoryStarCount = formatRepositoryStarCount(match.repoStars)

    return (
        <SelectableSearchResult match={match} selectResult={selectResult} selectedResult={selectedResult}>
            {isActive => (
                <SearchResultLayout
                    isActive={isActive}
                    iconColumn={{
                        icon: FileDocumentIcon,
                        repoName: match.repository,
                    }}
                    infoColumn={
                        <>
                            {lastSyncedTime && <LastSyncedIcon lastSyncedTime={lastSyncedTime} />}
                            {formattedRepositoryStarCount && (
                                <>
                                    <SearchResultStar />
                                    {formattedRepositoryStarCount}
                                </>
                            )}
                        </>
                    }
                >
                    <RepoName repoName={match.repository} suffix={match.path} />
                </SearchResultLayout>
            )}
        </SelectableSearchResult>
    )
}
