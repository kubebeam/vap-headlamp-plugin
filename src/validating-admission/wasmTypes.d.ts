declare global {
  export interface Window {
    Go: any;
    AdmissionEval: (policy: string, object: string, params: string) => string;
  }
}
export {};
