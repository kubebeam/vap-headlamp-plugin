import {
  Link as HeadlampLink,
  SectionBox,
  Table as HeadlampTable,
} from '@kinvolk/headlamp-plugin/lib/CommonComponents';
import { KubeObject } from '@kinvolk/headlamp-plugin/lib/lib/k8s/cluster.js';
import { useState } from 'react';
import { RoutingPath } from '..';
import { validatingAdmissionPolicyClass } from '../model';

export function ValidatingAdmissionPolicyList() {
  const [validatingAdmissionPolicyObjects, setValidatingAdmissionPolicies] =
    useState<KubeObject>(null);

  validatingAdmissionPolicyClass.useApiList(setValidatingAdmissionPolicies);

  if (!validatingAdmissionPolicyObjects) {
    return <div></div>;
  }

  return (
    <>
      <SectionBox title="Validating Admission Policies">
        <HeadlampTable
          data={validatingAdmissionPolicyObjects}
          columns={[
            {
              header: 'Name',
              accessorFn: (policy: any) => policy.metadata.name,
              Cell: ({ cell, row }: any) => (
                <HeadlampLink
                  routeName={RoutingPath.Policy}
                  params={{
                    name: row.original.metadata.name,
                  }}
                >
                  {cell.getValue()}
                </HeadlampLink>
              ),
            },
            {
              header: 'Control',
              accessorFn: (policy: any) =>
                policy.metadata.labels ? policy.metadata.labels['controlId'] : '',
            },
          ]}
        />
      </SectionBox>
    </>
  );
}
