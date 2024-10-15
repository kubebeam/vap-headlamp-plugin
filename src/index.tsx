/* 
  Registration of menu and routes in headlamp. 
*/
import { registerRoute, registerSidebarEntry } from '@kinvolk/headlamp-plugin/lib';

export namespace RoutingPath {
  export const Policies = '/vap/policies';
  export const Policy = '/vap/policies/:name';
}

// ValidationAdmissionPolicy main sidebar
registerSidebarEntry({
  parent: null,
  name: 'vap',
  label: 'Admission Policies',
  icon: 'mdi:shield-search',
  url: RoutingPath.Policies,
});

registerSidebarEntry({
  parent: 'vap',
  name: 'vap-policies',
  label: 'Policies',
  url: RoutingPath.Policies,
});

import { ValidatingAdmissionPolicyList } from './validating-admission/ValidatingAdmissionPolicyList';

registerRoute({
  path: RoutingPath.Policies,
  parent: 'vap',
  sidebar: 'vap-policies',
  component: () => <ValidatingAdmissionPolicyList />,
  exact: true,
  name: 'Validation Admission Policies',
});

import { ValidatingAdmissionPolicy } from './validating-admission/ValidatingAdmissionPolicy';

registerRoute({
  path: RoutingPath.Policy,
  parent: 'vap',
  sidebar: 'vap-policies',
  component: () => <ValidatingAdmissionPolicy />,
  exact: true,
  name: 'Validation Admission Policies',
});
