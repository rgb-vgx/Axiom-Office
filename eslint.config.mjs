import js from "@eslint/js";
import globals from "globals";

// Ghi chu: package.json o goc repo chi de lint JS (ESLint), khong tham gia phat hanh.
// Dung file cau hinh .mjs (khong dat "type": "module" vao package.json) de khong doi
// cach node hieu cac file .js hien co cua repo.

// Bo luat bat loi that. KHONG bat luat tham my/dinh dang.
const bugRules = {
  ...js.configs.recommended.rules,
  // Add-in nuot loi cua host la CHU Y (hang tram cho try/catch rong), nen khong bat
  // "tham so catch khong dung" va khong bat "khoi catch rong".
  "no-unused-vars": ["error", { caughtErrors: "none" }],
  "no-empty": ["error", { allowEmptyCatch: true }],
};

export default [
  // ESLint 9 chi tu bo qua node_modules + .git. dist/ va .ua/ la thu muc da bi git bo qua nhung
  // VAN chua ban sao .js (dist/.../wps-addin/ la ban build cua chinh 4 file add-in; .ua/ la thung rac
  // cua mot cong cu) nen phai loai o day, khong thi lint se bao trung.
  { ignores: ["dist/**", ".ua/**", ".kilo/**"] },
  {
    files: ["src/AxiomOffice.WPS/**/*.js"],
    languageOptions: {
      ecmaVersion: 5, // chi rang buoc CU PHAP; API cua Chrome 87 (Promise, globalThis) van dung duoc
      sourceType: "script",
      globals: {
        ...globals.browser,
        wps: "readonly", // API host WPS do CEF tiem vao
        AxiomCommands: "writable", // do chinh commands.js dat len root, main.js/pane-core.js doc lai
        AxiomPaneCore: "readonly", // pane-core.js dat len root, pane.js/mini.js doc lai
        AxiomPaneDom: "readonly", // pane-dom.js dat len root, pane.js/mini.js doc lai
        Promise: "readonly",
        globalThis: "readonly",
        module: "writable", // duong CommonJS cho node --test, luon nam trong guard typeof
      },
    },
    rules: bugRules,
  },
  {
    files: ["tests/wps/**/*.mjs", "eslint.config.mjs"],
    languageOptions: { ecmaVersion: 2024, sourceType: "module", globals: { ...globals.node } },
    rules: bugRules,
  },
];
