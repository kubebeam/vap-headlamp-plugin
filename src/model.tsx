import { makeCustomResourceClass } from '@kinvolk/headlamp-plugin/lib/lib/k8s/crd';

const admissionregistrationGroupVersions = [
  { group: 'admissionregistration.k8s.io', version: 'v1' },
];

export const validatingAdmissionPolicyClass = makeCustomResourceClass({
  apiInfo: admissionregistrationGroupVersions,
  isNamespaced: false,
  singularName: 'validatingadmissionpolicy',
  pluralName: 'validatingadmissionpolicies',
});
