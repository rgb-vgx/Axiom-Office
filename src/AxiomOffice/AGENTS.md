# AGENTS.md — src/AxiomOffice

Rules for the Windows Office add-in (C# / COM).

## Build and target

- .NET Framework 4.8, C# 7.3, x64. Keep the language level; do not use newer C#
  syntax that the Roslyn `csc` will reject.
- No NuGet package references: use the .NET Framework assemblies already referenced
  by the project. Do not add a package restore step.
- Build with `scripts/build.ps1` (it locates Roslyn `csc`, stages output, embeds
  localization resources, and builds Agent Core). C# cannot be built on Linux.

## COM

- All COM calls from bridge threads go through `ComGate`. The Office object model is
  not thread-safe; bypassing the gate has crashed Word.
- Host-facing interfaces use
  `[InterfaceType(ComInterfaceType.InterfaceIsIDispatch)]` plus explicit `[DispId]`.
  Do not declare raw IUnknown interfaces; that crashes the host.
- Wrap COM calls in try/catch. Keep the retry set for busy/rejected hosts:
  `RPC_E_CALL_REJECTED` (`0x80010001`), `0x8001010A`, and `0x800AC472`
  (plus the existing `0x80010002`).

## User environment

- Write only under `HKCU` (`HKCU\Software\AxiomOffice`) and user-profile paths. Never
  require admin or write machine-wide registry keys.
- Never kill, restart, or close the user's Office/WPS processes or documents. Live
  tests must start their own instances; `build.ps1 -Kill` is for an explicit local
  build, not for test automation.
- Never print tokens or API keys to logs, UI, tests, or commits.
