import {Page} from 'argo-ui/src/components/page/page';
import {SlidingPanel} from 'argo-ui/src/components/sliding-panel/sliding-panel';
import * as React from 'react';
import {useContext, useEffect, useRef, useState} from 'react';
import {RouteComponentProps} from 'react-router-dom';

import {uiUrl} from '../shared/base';
import {ErrorNotice} from '../shared/components/error-notice';
import {ExampleManifests} from '../shared/components/example-manifests';
import {InfoIcon} from '../shared/components/fa-icons';
import {Loading} from '../shared/components/loading';
import {PaginationPanel} from '../shared/components/pagination-panel';
import {TimestampSwitch} from '../shared/components/timestamp';
import {ZeroState} from '../shared/components/zero-state';
import {Context} from '../shared/context';
import {Footnote} from '../shared/footnote';
import {historyUrl} from '../shared/history';
import {WorkflowTemplate} from '../shared/models';
import * as nsUtils from '../shared/namespaces';
import {Pagination, parseLimit} from '../shared/pagination';
import {ScopedLocalStorage} from '../shared/scoped-local-storage';
import {services} from '../shared/services';
import {useCollectEvent} from '../shared/use-collect-event';
import {useQueryParams} from '../shared/use-query-params';
import useTimestamp, {TIMESTAMP_KEYS} from '../shared/use-timestamp';
import {WorkflowTemplateCreator} from './workflow-template-creator';
import {WorkflowTemplateFilters} from './workflow-template-filters';
import {WorkflowTemplateRow} from './workflow-template-row';

import './workflow-template-list.scss';

const learnMore = <a href='https://argo-workflows.readthedocs.io/en/latest/workflow-templates/'>Learn more</a>;

export function WorkflowTemplateList({match, location, history}: RouteComponentProps<any>) {
    // boiler-plate
    const isFirstRender = useRef(true);
    const queryParams = new URLSearchParams(location.search);
    const {navigation} = useContext(Context);

    const storage = new ScopedLocalStorage('WorkflowTemplateListOptions');
    const savedOptions = storage.getItem('paginationLimit', 0);

    // state for URL and query parameters
    const [namespace, setNamespace] = useState(nsUtils.getNamespace(match.params.namespace) || '');
    const [sidePanel, setSidePanel] = useState(queryParams.get('sidePanel') === 'true');
    const [namePattern, setNamePattern] = useState('');
    const [labels, setLabels] = useState([]);
    const [pagination, setPagination] = useState<Pagination>({
        offset: queryParams.get('offset'),
        limit: parseLimit(queryParams.get('limit')) || savedOptions.paginationLimit || 500
    });

    useEffect(
        useQueryParams(history, p => {
            setSidePanel(p.get('sidePanel') === 'true');
        }),
        [history]
    );

    useEffect(() => {
        if (isFirstRender.current) {
            isFirstRender.current = false;
            return;
        }
        history.push(
            historyUrl('workflow-templates' + (nsUtils.getManagedNamespace() ? '' : '/{namespace}'), {
                namespace,
                sidePanel
            })
        );
    }, [namespace, sidePanel]);

    // internal state
    const [error, setError] = useState<Error>();
    const [templates, setTemplates] = useState<WorkflowTemplate[]>();
    useEffect(() => {
        services.workflowTemplate
            .list(namespace, labels, namePattern, pagination)
            .then(list => {
                setPagination({...pagination, nextOffset: list.metadata.continue});
                setTemplates(list.items || []);
            })
            .then(() => setError(null))
            .catch(setError);
    }, [namespace, namePattern, labels.toString(), pagination.offset, pagination.limit]); // referential equality, so use values, not refs
    useEffect(() => {
        storage.setItem('paginationLimit', pagination.limit, 0);
    }, [pagination.limit]);

    useCollectEvent('openedWorkflowTemplateList');

    const [storedDisplayISOFormat, setStoredDisplayISOFormat] = useTimestamp(TIMESTAMP_KEYS.WORKFLOW_TEMPLATE_LIST_CREATION);

    return (
        <Page
            title='Workflow Templates'
            toolbar={{
                breadcrumbs: [
                    {title: 'Workflow Templates', path: uiUrl('workflow-templates')},
                    {title: namespace, path: uiUrl('workflow-templates/' + namespace)}
                ],
                actionMenu: {
                    items: [
                        {
                            title: 'Create New Workflow Template',
                            iconClassName: 'fa fa-plus',
                            action: () => setSidePanel(true)
                        }
                    ]
                }
            }}>
            <div className='row'>
                <div className='columns small-12 xlarge-2'>
                    <div>
                        <WorkflowTemplateFilters
                            templates={templates || []}
                            namespace={namespace}
                            namePattern={namePattern}
                            labels={labels}
                            onChange={(namespaceValue: string, namePatternValue: string, labelsValue: string[]) => {
                                setNamespace(namespaceValue);
                                setNamePattern(namePatternValue);
                                setLabels(labelsValue);
                                setPagination({...pagination, offset: ''});
                            }}
                        />
                    </div>
                </div>
                <div className='columns small-12 xlarge-10'>
                    <ErrorNotice error={error} />
                    {!templates ? (
                        <Loading />
                    ) : templates.length === 0 ? (
                        <ZeroState title='No workflow templates'>
                            <p>You can create new templates here or using the CLI.</p>
                            <p>
                                <ExampleManifests />. {learnMore}.
                            </p>
                        </ZeroState>
                    ) : (
                        <>
                            <div className='argo-table-list'>
                                <div className='row argo-table-list__head'>
                                    <div className='columns small-1' />
                                    <div className='columns small-5'>NAME</div>
                                    <div className='columns small-3'>NAMESPACE</div>
                                    <div className='columns small-3'>
                                        CREATED <TimestampSwitch storedDisplayISOFormat={storedDisplayISOFormat} setStoredDisplayISOFormat={setStoredDisplayISOFormat} />
                                    </div>
                                </div>
                                {templates.map(t => {
                                    return <WorkflowTemplateRow workflow={t} displayISOFormat={storedDisplayISOFormat} key={`{t.metadata.namespace}/${t.metadata.name}`} />;
                                })}
                            </div>
                            <Footnote>
                                <InfoIcon /> Workflow templates are reusable templates you can create new workflows from. <ExampleManifests />. {learnMore}.
                            </Footnote>
                            <PaginationPanel onChange={setPagination} pagination={pagination} numRecords={null} />
                        </>
                    )}
                </div>
            </div>
            <SlidingPanel isShown={sidePanel} onClose={() => setSidePanel(false)}>
                <WorkflowTemplateCreator namespace={namespace} onCreate={wf => navigation.goto(uiUrl(`workflow-templates/${wf.metadata.namespace}/${wf.metadata.name}`))} />
            </SlidingPanel>
        </Page>
    );
}