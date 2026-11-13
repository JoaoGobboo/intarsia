/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Set by the Makefile; absent when Vite is started directly. */
  readonly VITE_APP_VERSION?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
