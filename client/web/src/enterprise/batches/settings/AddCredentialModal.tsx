import React, { useCallback, useState } from 'react'

import classNames from 'classnames'

import { ErrorAlert } from '@sourcegraph/branded/src/components/alerts'
import { Form } from '@sourcegraph/branded/src/components/Form'
import { Button, Modal, Link, Typography, Text, Input } from '@sourcegraph/wildcard'

import { LoaderButton } from '../../../components/LoaderButton'
import { ExternalServiceKind, Scalars } from '../../../graphql-operations'

import { useCreateBatchChangesCredential } from './backend'
import { CodeHostSshPublicKey } from './CodeHostSshPublicKey'
import { ModalHeader } from './ModalHeader'

import styles from './AddCredentialModal.module.scss'

export interface AddCredentialModalProps {
    onCancel: () => void
    afterCreate: () => void
    userID: Scalars['ID'] | null
    externalServiceKind: ExternalServiceKind
    externalServiceURL: string
    requiresSSH: boolean
    requiresUsername: boolean

    /** For testing only. */
    initialStep?: Step
}

const HELP_TEXT_LINK_URL = 'https://docs.sourcegraph.com/batch_changes/quickstart#configure-code-host-credentials'

const scopeRequirements: Record<ExternalServiceKind, JSX.Element> = {
    [ExternalServiceKind.GITHUB]: (
        <span>
            with the <Typography.Code>repo</Typography.Code>, <Typography.Code>read:org</Typography.Code>,{' '}
            <Typography.Code>user:email</Typography.Code>, <Typography.Code>read:discussion</Typography.Code>, and{' '}
            <Typography.Code>workflow</Typography.Code> scopes.
        </span>
    ),
    [ExternalServiceKind.GITLAB]: (
        <span>
            with <Typography.Code>api</Typography.Code>, <Typography.Code>read_repository</Typography.Code>, and{' '}
            <Typography.Code>write_repository</Typography.Code> scopes.
        </span>
    ),
    [ExternalServiceKind.BITBUCKETSERVER]: (
        <span>
            with <Typography.Code>write</Typography.Code> permissions on the project and repository level.
        </span>
    ),
    [ExternalServiceKind.BITBUCKETCLOUD]: (
        <span>
            with <Typography.Code>account:read</Typography.Code>, <Typography.Code>repo:write</Typography.Code>,{' '}
            <Typography.Code>pr:write</Typography.Code>, and <Typography.Code>pipeline:read</Typography.Code>{' '}
            permissions.
        </span>
    ),

    // These are just for type completeness and serve as placeholders for a bright future.
    [ExternalServiceKind.GERRIT]: <span>Unsupported</span>,
    [ExternalServiceKind.GITOLITE]: <span>Unsupported</span>,
    [ExternalServiceKind.GOMODULES]: <span>Unsupported</span>,
    [ExternalServiceKind.PYTHONPACKAGES]: <span>Unsupported</span>,
    [ExternalServiceKind.JVMPACKAGES]: <span>Unsupported</span>,
    [ExternalServiceKind.NPMPACKAGES]: <span>Unsupported</span>,
    [ExternalServiceKind.PERFORCE]: <span>Unsupported</span>,
    [ExternalServiceKind.PHABRICATOR]: <span>Unsupported</span>,
    [ExternalServiceKind.AWSCODECOMMIT]: <span>Unsupported</span>,
    [ExternalServiceKind.PAGURE]: <span>Unsupported</span>,
    [ExternalServiceKind.OTHER]: <span>Unsupported</span>,
}

type Step = 'add-token' | 'get-ssh-key'

export const AddCredentialModal: React.FunctionComponent<React.PropsWithChildren<AddCredentialModalProps>> = ({
    onCancel,
    afterCreate,
    userID,
    externalServiceKind,
    externalServiceURL,
    requiresSSH,
    requiresUsername,
    initialStep = 'add-token',
}) => {
    const labelId = 'addCredential'
    const [credential, setCredential] = useState<string>('')
    const [sshPublicKey, setSSHPublicKey] = useState<string>()
    const [username, setUsername] = useState<string>('')
    const [step, setStep] = useState<Step>(initialStep)

    const onChangeCredential = useCallback<React.ChangeEventHandler<HTMLInputElement>>(event => {
        setCredential(event.target.value)
    }, [])

    const onChangeUsername = useCallback<React.ChangeEventHandler<HTMLInputElement>>(event => {
        setUsername(event.target.value)
    }, [])

    const [createBatchChangesCredential, { loading, error }] = useCreateBatchChangesCredential()

    const onSubmit = useCallback<React.FormEventHandler>(
        async event => {
            event.preventDefault()

            try {
                const { data } = await createBatchChangesCredential({
                    variables: {
                        user: userID,
                        credential,
                        username: requiresUsername ? username : null,
                        externalServiceKind,
                        externalServiceURL,
                    },
                })

                if (requiresSSH && data?.createBatchChangesCredential.sshPublicKey) {
                    setSSHPublicKey(data?.createBatchChangesCredential.sshPublicKey)
                    setStep('get-ssh-key')
                } else {
                    afterCreate()
                }
            } catch (error) {
                console.error(error)
            }
        },
        [
            createBatchChangesCredential,
            userID,
            credential,
            requiresUsername,
            username,
            externalServiceKind,
            externalServiceURL,
            requiresSSH,
            afterCreate,
        ]
    )

    const patLabel =
        externalServiceKind === ExternalServiceKind.BITBUCKETCLOUD ? 'App password' : 'Personal access token'

    return (
        <Modal onDismiss={onCancel} aria-labelledby={labelId}>
            <div className="test-add-credential-modal">
                <ModalHeader
                    id={labelId}
                    externalServiceKind={externalServiceKind}
                    externalServiceURL={externalServiceURL}
                />
                {requiresSSH && (
                    <div className="d-flex w-100 justify-content-between mb-4">
                        <div className="flex-grow-1 mr-2">
                            <Text className={classNames('mb-0 py-2', step === 'get-ssh-key' && 'text-muted')}>
                                1. Add token
                            </Text>
                            <div
                                className={classNames(
                                    styles.addCredentialModalModalStepRuler,
                                    styles.addCredentialModalModalStepRulerPurple
                                )}
                            />
                        </div>
                        <div className="flex-grow-1 ml-2">
                            <Text className={classNames('mb-0 py-2', step === 'add-token' && 'text-muted')}>
                                2. Get SSH Key
                            </Text>
                            <div
                                className={classNames(
                                    styles.addCredentialModalModalStepRuler,
                                    step === 'add-token' && styles.addCredentialModalModalStepRulerGray,
                                    step === 'get-ssh-key' && styles.addCredentialModalModalStepRulerBlue
                                )}
                            />
                        </div>
                    </div>
                )}
                {step === 'add-token' && (
                    <>
                        {error && <ErrorAlert error={error} />}
                        <Form onSubmit={onSubmit}>
                            <div className="form-group">
                                {requiresUsername && (
                                    <>
                                        <Typography.Label htmlFor="username">Username</Typography.Label>
                                        <Input
                                            id="username"
                                            name="username"
                                            autoComplete="off"
                                            className="mb-2"
                                            required={true}
                                            spellCheck="false"
                                            minLength={1}
                                            value={username}
                                            onChange={onChangeUsername}
                                        />
                                    </>
                                )}
                                <Typography.Label htmlFor="token">{patLabel}</Typography.Label>
                                <Input
                                    id="token"
                                    name="token"
                                    type="password"
                                    autoComplete="off"
                                    data-testid="test-add-credential-modal-input"
                                    required={true}
                                    spellCheck="false"
                                    minLength={1}
                                    value={credential}
                                    onChange={onChangeCredential}
                                />
                                <Text className="form-text">
                                    <Link
                                        to={HELP_TEXT_LINK_URL}
                                        rel="noreferrer noopener"
                                        target="_blank"
                                        aria-label={`Follow our docs to learn how to create a new ${patLabel.toLocaleLowerCase()} on this code host`}
                                    >
                                        Create a new {patLabel.toLocaleLowerCase()}
                                    </Link>{' '}
                                    {scopeRequirements[externalServiceKind]}
                                </Text>
                            </div>
                            <div className="d-flex justify-content-end">
                                <Button
                                    disabled={loading}
                                    className="mr-2"
                                    onClick={onCancel}
                                    outline={true}
                                    variant="secondary"
                                >
                                    Cancel
                                </Button>
                                <LoaderButton
                                    type="submit"
                                    disabled={loading || credential.length === 0}
                                    className="test-add-credential-modal-submit"
                                    variant="primary"
                                    loading={loading}
                                    alwaysShowLabel={true}
                                    label={requiresSSH ? 'Next' : 'Add credential'}
                                />
                            </div>
                        </Form>
                    </>
                )}
                {step === 'get-ssh-key' && (
                    <>
                        <Text>
                            An SSH key has been generated for your batch changes code host connection. Copy the public
                            key below and enter it on your code host.
                        </Text>
                        <CodeHostSshPublicKey externalServiceKind={externalServiceKind} sshPublicKey={sshPublicKey!} />
                        <Button
                            className="test-add-credential-modal-submit float-right"
                            onClick={afterCreate}
                            variant="primary"
                        >
                            Finish
                        </Button>
                    </>
                )}
            </div>
        </Modal>
    )
}
