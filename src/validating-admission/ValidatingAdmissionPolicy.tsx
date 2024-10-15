import './wasm_exec.js';
import './wasmTypes.d.ts';
import { NameValueTable, SectionBox } from '@kinvolk/headlamp-plugin/lib/CommonComponents';
import { KubeObject } from '@kinvolk/headlamp-plugin/lib/lib/k8s/cluster';
import * as yaml from 'js-yaml';
import { useEffect, useState } from 'react';
import { getURLSegments } from '../common/url';
import { validatingAdmissionPolicyClass } from '../model';

export function ValidatingAdmissionPolicy() {
  const [name] = getURLSegments(-1);
  const [isWasmLoaded, setIsWasmLoaded] = useState(false);
  const [validatingAdmissionPolicyObject, setValidatingAdmissionPolicy] =
    useState<KubeObject>(null);

  useEffect(() => {
    async function loadWasm(): Promise<void> {
      const goWasm = new window.Go();
      const result = await WebAssembly.instantiateStreaming(
        fetch(`/plugins/vap-plugin/main.wasm`),
        goWasm.importObject
      );
      goWasm.run(result.instance);
      setIsWasmLoaded(true);
    }

    loadWasm();
  }, []);

  validatingAdmissionPolicyClass.useApiGet(setValidatingAdmissionPolicy, name);

  if (!validatingAdmissionPolicyObject || !isWasmLoaded) {
    return <div></div>;
  }

  const validatingAdmissionPolicy: any = validatingAdmissionPolicyObject.jsonData;

  return (
    <>
      <div>
        {isWasmLoaded && <p>Wasm Loaded</p>}
        {!isWasmLoaded && <p>Wasm not Loaded</p>}
      </div>
      <SectionBox title="Validating Admission Policy">
        <NameValueTable
          rows={[
            {
              name: 'Name',
              value: validatingAdmissionPolicy.metadata.name,
            },
            {
              name: 'Expression',
              value: validatingAdmissionPolicy.spec.validations.map((v: any) => v.message).join(''),
            },
          ]}
        />
      </SectionBox>

      <AdmissionEvaluator validatingAdmissionPolicy={validatingAdmissionPolicy} />
    </>
  );
}

function AdmissionEval(expr: string, object: string) {
  return new Promise<string>(resolve => {
    const res = window.AdmissionEval(expr, object);
    resolve(res);
  });
}

function AdmissionEvaluator(props: { validatingAdmissionPolicy: any }) {
  const { validatingAdmissionPolicy } = props;

  // strip status
  const strippedResource: any = Object.fromEntries(
    Object.entries(validatingAdmissionPolicy).filter(([key]) => key !== 'status')
  );
  // strip managedFields
  strippedResource.metadata = Object.fromEntries(
    Object.entries(strippedResource.metadata).filter(([key]) => key !== 'managedFields')
  );
  // strip from annotations
  strippedResource.metadata.annotations = Object.fromEntries(
    Object.entries(strippedResource.metadata.annotations).filter(
      ([key]) => key !== 'kubectl.kubernetes.io/last-applied-configuration'
    )
  );

  const [validatingAdmissionPolicyYAML, setValidatingAdmissionPolicyYAML] = useState(
    yaml.dump(strippedResource)
  );

  const [resource, setResource] = useState(
    `apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    deployment.kubernetes.io/revision: "1"
  creationTimestamp: "2023-10-02T15:26:06Z"
  generation: 1
  labels:
    app: kubernetes-bootcamp
  name: kubernetes-bootcamp
  namespace: default
  resourceVersion: "246826"
  uid: dcdda63b-1611-467d-8927-43e3c73bc963
spec:
  progressDeadlineSeconds: 600
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: kubernetes-bootcamp
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
    type: RollingUpdate
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: kubernetes-bootcamp
    spec:
      containers:
      - image: gcr.io/google-samples/kubernetes-bootcamp:v1
        imagePullPolicy: IfNotPresent
        name: kubernetes-bootcamp
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
`.trim()
  );
  const [result, setResult] = useState('');

  const handleExpressionChange = (event: any) => {
    setValidatingAdmissionPolicyYAML(event.target.value);
  };

  const handleResourceChange = (event: any) => {
    setResource(event.target.value);
  };

  const handleAdmissionClick = async () => {
    const output = await AdmissionEval(validatingAdmissionPolicyYAML, resource);
    setResult(output);
  };

  return (
    <div>
      <textarea
        value={validatingAdmissionPolicyYAML}
        onChange={handleExpressionChange}
        rows={40}
        cols={100}
      />
      <textarea value={resource} onChange={handleResourceChange} rows={40} cols={50} />
      <textarea value={result} rows={20} cols={100} />
      <button onClick={handleAdmissionClick}>Evaluate</button>
    </div>
  );
}
