import './wasm_exec.js';
import './wasmTypes.d.ts';
import { ApiProxy, KubeObject } from '@kinvolk/headlamp-plugin/lib';
import { NameValueTable, SectionBox } from '@kinvolk/headlamp-plugin/lib/CommonComponents';
import Editor from '@monaco-editor/react';
import * as yaml from 'js-yaml';
import { useEffect, useRef, useState } from 'react';
import { getURLSegments } from '../common/url';
import { validatingAdmissionPolicyClass } from '../model';
import { sampleDeployment } from './samples';

export function ValidatingAdmissionPolicy() {
  const [name] = getURLSegments(-1);
  const [validatingAdmissionPolicyObject, setValidatingAdmissionPolicy] =
    useState<KubeObject>(null);

  validatingAdmissionPolicyClass.useApiGet(setValidatingAdmissionPolicy, name);

  if (!validatingAdmissionPolicyObject) {
    return <div></div>;
  }

  const validatingAdmissionPolicy: any = validatingAdmissionPolicyObject.jsonData;

  return (
    <>
      <SectionBox title="Validating Admission Policy">
        <NameValueTable
          rows={[
            {
              name: 'Name',
              value: validatingAdmissionPolicy.metadata.name,
            },
            {
              name: 'Expression',
              value: validatingAdmissionPolicy.spec.validations
                .map((v: any) => v.message)
                .join(', '),
            },
          ]}
        />
      </SectionBox>

      <AdmissionEvaluator validatingAdmissionPolicy={validatingAdmissionPolicy} />
    </>
  );
}

function AdmissionEval(policy: string, object: string, params: string) {
  return new Promise<string>(resolve => {
    const res = window.AdmissionEval(policy, object, params);
    resolve(res);
  });
}

function cleanPolicyObject(validatingAdmissionPolicy: any): any {
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
  return strippedResource;
}

function AdmissionEvaluator(props: { validatingAdmissionPolicy: any }) {
  const { validatingAdmissionPolicy } = props;
  const [params, setValidationParams] = useState(null);
  const [result, setResult] = useState('');
  const policyEditorRef = useRef(null);
  const resourceEditorRef = useRef(null);

  // Get params if defined
  if (validatingAdmissionPolicy.spec.paramKind?.apiVersion) {
    const pluralName = validatingAdmissionPolicy.spec.paramKind?.kind.toLowerCase() + 's';

    useEffect(() => {
      const groupVersion = validatingAdmissionPolicy.spec.paramKind?.apiVersion;
      ApiProxy.request(`/apis/${groupVersion}/${pluralName}`).then((result: any) => {
        setValidationParams(result.items[0]);
      });
    }, []);
    if (!params) {
      return <></>;
    }
  }

  const submitEvaluation = async () => {
    const output = await AdmissionEval(
      policyEditorRef.current?.getValue(),
      resourceEditorRef.current?.getValue(),
      yaml.dump(params)
    );
    const yamlString = yaml.dump(JSON.parse(output));
    setResult(yamlString);
  };

  function handleMountPolicyEditor(editor: any) {
    policyEditorRef.current = editor;
    editor.onDidChangeModelContent(submitEvaluation);
  }

  function handleMountResourceEditor(editor: any) {
    resourceEditorRef.current = editor;
    editor.onDidChangeModelContent(submitEvaluation);
  }

  return (
    <>
      <textarea value={result} rows={10} cols={100} readOnly />
      <br />
      <button onClick={submitEvaluation}>Evaluate</button>
      <SectionBox paddingTop={2} title="Validating Admission Policy">
        <Editor
          language="yaml"
          onMount={handleMountPolicyEditor}
          theme={localStorage.headlampThemePreference === 'dark' ? 'vs-dark' : ''}
          value={yaml.dump(cleanPolicyObject(validatingAdmissionPolicy))}
          width={800}
          height={600}
        />
      </SectionBox>
      <SectionBox paddingTop={2} title="Resource">
        <Editor
          language="yaml"
          onMount={handleMountResourceEditor}
          theme={localStorage.headlampThemePreference === 'dark' ? 'vs-dark' : ''}
          value={sampleDeployment}
          width={800}
          height={600}
        />
      </SectionBox>
    </>
  );
}
