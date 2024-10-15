declare global {
  export interface Window {
    Go: any;
    AdmissionEval: (s1: string, s2: string) => string;
  }
}
export {};
