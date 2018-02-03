import CloseIcon from '@sourcegraph/icons/lib/Close'
import ErrorIcon from '@sourcegraph/icons/lib/Error'
import * as React from 'react'
import { Observable } from 'rxjs/Observable'
import { fromEvent } from 'rxjs/observable/fromEvent'
import { catchError } from 'rxjs/operators/catchError'
import { filter } from 'rxjs/operators/filter'
import { map } from 'rxjs/operators/map'
import { Subscription } from 'rxjs/Subscription'
import { configurationCascade } from '../settings/configuration'
import { eventLogger } from '../tracking/eventLogger'

export interface SavedQueryFields {
    description: string
    query: string
    subject: GQLID
    showOnHomepage: boolean
    notify: boolean
    notifySlack: boolean
    notifyUsers: string[]
    notifyOrganizations: string[]
}

interface Props {
    defaultValues?: Partial<SavedQueryFields>
    title?: string
    submitLabel: string
    onSubmit: (fields: SavedQueryFields) => Observable<void>
    onDidCommit: () => void
    onDidCancel: () => void
}

interface State {
    values: SavedQueryFields

    subjectOptions: GQL.ConfigurationSubject[]
    isSubmitting: boolean
    isFocused: boolean
    error?: any
    sawUnsupportedNotifyQueryWarning: boolean
}

export class SavedQueryForm extends React.Component<Props, State> {
    private handleDescriptionChange = this.createInputChangeHandler('description')
    private handleSubjectChange = this.createInputChangeHandler('subject')
    private handleShowOnHomeChange = this.createInputChangeHandler('showOnHomepage')
    private handleNotifyChange = this.createInputChangeHandler('notify')
    private handleNotifySlackChange = this.createInputChangeHandler('notifySlack')

    private subscriptions = new Subscription()

    constructor(props: Props) {
        super(props)

        const { defaultValues } = props

        this.state = {
            values: {
                query: (defaultValues && defaultValues.query) || '',
                description: (defaultValues && defaultValues.description) || '',
                subject: (defaultValues && defaultValues.subject) || '',
                showOnHomepage: !!(defaultValues && defaultValues.showOnHomepage),
                notify: !!(defaultValues && defaultValues.notify),
                notifySlack: !!(defaultValues && defaultValues.notifySlack),
                notifyUsers: (defaultValues && defaultValues.notifyUsers) || [],
                notifyOrganizations: (defaultValues && defaultValues.notifyOrganizations) || [],
            },
            subjectOptions: [],
            isSubmitting: false,
            isFocused: false,
            sawUnsupportedNotifyQueryWarning: false,
        }
    }

    public componentDidMount(): void {
        this.subscriptions.add(
            configurationCascade.pipe(map(({ subjects }) => subjects)).subscribe(subjects => {
                const subject = subjects.find(s => !!s.id)

                this.setState(state => ({
                    subjectOptions: subjects,
                    values: {
                        ...state.values,
                        subject: state.values.subject || (subject && subject.id) || '',
                    },
                }))
            })
        )

        this.subscriptions.add(
            fromEvent<KeyboardEvent>(window, 'keydown')
                .pipe(filter(event => !this.state.isFocused && event.key === 'Escape' && !this.state.isSubmitting))
                .subscribe(() => this.props.onDidCancel())
        )
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element {
        const { onDidCancel, title, submitLabel } = this.props
        const {
            values: {
                query,
                description,
                subject,
                showOnHomepage,
                notify,
                notifySlack,
                notifyUsers,
                notifyOrganizations,
            },
            subjectOptions,
            isSubmitting,
            error,
        } = this.state
        const savingToOrg = this.savingToOrg()

        return (
            <form className="saved-query-form" onSubmit={this.handleSubmit}>
                <div className="saved-query-form__title-row">
                    {title ? (
                        <h1 className="saved-query-form__title saved-query-form__spacer">{title}</h1>
                    ) : (
                        <div className="saved-query-form__spacer" />
                    )}
                    <button className="btn btn-icon saved-query-form__close" onClick={onDidCancel}>
                        <CloseIcon />
                    </button>
                </div>
                <span>Search query</span>
                <input
                    type="text"
                    name="query"
                    className="form-control"
                    placeholder="Query"
                    onChange={this.handleQueryChange}
                    value={query || ''}
                    autoCorrect="off"
                    spellCheck={false}
                    autoCapitalize="off"
                    autoFocus={!query}
                    onFocus={this.handleInputFocus}
                    onBlur={this.handleInputBlur}
                />
                <span>Description of this search</span>
                <input
                    type="text"
                    name="description"
                    className="form-control"
                    placeholder="Description"
                    onChange={this.handleDescriptionChange}
                    value={description || ''}
                    required={true}
                    autoFocus={!!query && !description}
                    onFocus={this.handleInputFocus}
                    onBlur={this.handleInputBlur}
                />
                <span>Save location</span>
                <div className="saved-query-form__save-location">
                    {subjectOptions
                        .filter(
                            (subjectOption: GQL.ConfigurationSubject): subjectOption is GQL.IOrg | GQL.IUser =>
                                subjectOption.__typename === 'Org' || subjectOption.__typename === 'User'
                        )
                        .map((subjectOption, i) => (
                            <span className="saved-query-form__save-location-options" key={i}>
                                <label>
                                    <input
                                        className="saved-query-form__save-location-input"
                                        onChange={this.handleSubjectChange}
                                        type="radio"
                                        value={subjectOption.id}
                                        checked={subject === subjectOption.id}
                                    />
                                    {configurationSubjectLabel(subjectOption)}
                                </label>
                            </span>
                        ))}
                </div>
                <div className="saved-query-form__save-location">
                    <span className="saved-query-form__save-location-options">
                        <label>
                            <input
                                className="saved-query-form__save-location-input"
                                type="checkbox"
                                defaultChecked={showOnHomepage}
                                onChange={this.handleShowOnHomeChange}
                            />
                            Show on homepage
                        </label>
                    </span>
                    <span className="saved-query-form__save-location-options">
                        <label data-tooltip={`Send email notifications to config owner (${this.saveTargetName()})`}>
                            <input
                                className="saved-query-form__save-location-input"
                                type="checkbox"
                                defaultChecked={notify}
                                onChange={this.handleNotifyChange}
                            />
                            Email notifications
                        </label>
                    </span>
                    <span className="saved-query-form__save-location-options">
                        <label
                            data-tooltip={
                                savingToOrg
                                    ? `Send slack notifications to config owner (${this.saveTargetName()})`
                                    : 'Must save to org settings to enable Slack notifications'
                            }
                        >
                            <input
                                className="saved-query-form__save-location-input"
                                type="checkbox"
                                defaultChecked={notifySlack}
                                onChange={this.handleNotifySlackChange}
                                disabled={!savingToOrg}
                            />
                            Slack notifications
                        </label>
                    </span>
                </div>
                {(notifyUsers.length > 0 || notifyOrganizations.length > 0) && (
                    <div className="saved-query-form__also-notifying">
                        Note: also notifying &nbsp;
                        <strong>
                            {notifyUsers
                                .map(user => `${user} (user)`)
                                .concat(notifyOrganizations.map(org => `${org} (org)`))
                                .join(', ')}
                        </strong>
                        &nbsp;
                        <span data-tooltip="See `notifyUsers` and `notifyOrganizations` in the JSON configuration">
                            due to manual configuration
                        </span>
                    </div>
                )}
                {this.isUnsupportedNotifyQuery(this.state.values) && (
                    <div>
                        Warning: non-commit searches do not currently support notifications. Consider adding `type:diff`
                        or `type:commit` to your query.
                    </div>
                )}
                <div className="saved-query-form__actions">
                    <button
                        type="submit"
                        className="btn btn-primary saved-query-form__button"
                        disabled={!(description && query && subject) || isSubmitting}
                    >
                        {submitLabel}
                    </button>
                    <button
                        type="reset"
                        className="btn btn-secondary saved-query-form__button saved-query-form__button-cancel"
                        disabled={isSubmitting}
                        onClick={onDidCancel}
                    >
                        Cancel
                    </button>
                </div>
                {error &&
                    !isSubmitting && (
                        <div className="saved-query-form__error">
                            <ErrorIcon className="icon-inline saved-query-form__error-icon" />
                            {error.message}
                        </div>
                    )}
            </form>
        )
    }

    /**
     * Tells if the query is unsupported for sending notifications.
     */
    private isUnsupportedNotifyQuery(v: SavedQueryFields): boolean {
        const notifying = v.notify || v.notifySlack || v.notifyUsers.length > 0 || v.notifyOrganizations.length > 0
        return notifying && !v.query.includes('type:diff') && !v.query.includes('type:commit')
    }

    private savingToOrg = () => {
        const chosen = this.state.subjectOptions.find(subjectOption => subjectOption.id === this.state.values.subject)
        return chosen && chosen.__typename === 'Org'
    }

    private saveTargetName = () => {
        const chosen = this.state.subjectOptions
            .filter(
                (subjectOption: GQL.ConfigurationSubject): subjectOption is GQL.IOrg | GQL.IUser =>
                    subjectOption.__typename === 'Org' || subjectOption.__typename === 'User'
            )
            .find(subjectOption => subjectOption.id === this.state.values.subject)
        return chosen && configurationSubjectLabel(chosen, true)
    }

    private handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
        event.preventDefault()

        this.setState({ isSubmitting: true })

        this.subscriptions.add(
            this.props
                .onSubmit(this.state.values)
                .pipe(
                    catchError(error => {
                        console.error(error)
                        this.setState({ error })

                        return []
                    })
                )
                .subscribe(this.props.onDidCommit)
        )
    }

    private handleInputFocus = (event: React.FocusEvent<HTMLInputElement>) => {
        this.setState(() => ({ isFocused: true }))
    }

    private handleInputBlur = (event: React.FocusEvent<HTMLInputElement>) => {
        this.setState(() => ({ isFocused: false }))
    }

    private handleQueryChange = (event: React.ChangeEvent<HTMLInputElement>) => {
        const newValues = {
            ...this.state.values,
            query: event.currentTarget.value,
        }
        if (!this.state.sawUnsupportedNotifyQueryWarning && this.isUnsupportedNotifyQuery(newValues)) {
            this.setState({ sawUnsupportedNotifyQueryWarning: true })
            eventLogger.log('SavedSearchUnsupportedNotifyQueryWarning')
        }

        const newQuery = event.currentTarget.value
        this.setState(prevState => ({
            values: {
                ...prevState.values,
                query: newQuery,
            },
        }))
    }

    /**
     * Returns an input change handler that updates the SavedQueryFields in the component's state
     *
     * @param key The key of saved query fields that a change of this input should update
     */
    private createInputChangeHandler(key: keyof SavedQueryFields): React.FormEventHandler<HTMLInputElement> {
        return event => {
            const { currentTarget: { value, checked, type } } = event

            this.setState(state => ({
                values: {
                    ...state.values,
                    [key]: type === 'checkbox' ? checked : value,
                },
            }))
        }
    }
}

function configurationSubjectLabel(s: GQL.IUser | GQL.IOrg, short?: boolean): string {
    switch (s.__typename) {
        case 'User':
            return short ? s.username : `${s.username} (user settings)`
        case 'Org':
            return short ? s.name : `${s.name} (org settings)`
    }
}
