# Licensing and Distribution

BillBot is licensed under LGPL-3.0-only.

SPDX-License-Identifier: LGPL-3.0-only

BillBot's license covers BillBot's own source code, including the Go backend, dashboard, connector interfaces, configuration logic, scripts, and documentation created for this project.

Third party projects are not relicensed by BillBot. They keep their own licenses and notices.

NapCatQQ
- BillBot must not vendor, embed, or redistribute NapCatQQ source code or binaries by default.
- NapCatQQ uses a custom Limited Redistribution License.
- The NapCatQQ license text includes non-commercial restrictions and requires permission for rights not explicitly granted.
- Because of that, BillBot treats NapCatQQ as an external connector dependency, not bundled project code.

Hermes Agent
- Hermes Agent is MIT licensed.
- BillBot integrates with Hermes as an optional external CLI runner.
- BillBot should not imply Hermes is part of BillBot or covered by BillBot's LGPL license.

Distribution model
- BillBot ships its own Go backend, dashboard, connector interface, and configuration logic.
- BillBot may provide install helpers that download NapCatQQ from the upstream source at runtime.
- BillBot may detect an existing NapCatQQ installation and connect to its OneBot HTTP/WebSocket endpoints.
- BillBot may generate configuration files for the user's local NapCatQQ instance.
- BillBot should not include NapCatQQ code, patched NapCatQQ bundles, NTQQ binaries, or prebuilt NapCatQQ images in its own release artifacts.

Recommended connector modes
1. external
   - User installs NapCatQQ separately.
   - BillBot only asks for HTTP and WebSocket URLs.
   - Lowest compliance risk.

2. installer
   - BillBot downloads the official NapCatQQ installer or release from upstream during setup.
   - User explicitly accepts NapCatQQ's license before installation.
   - BillBot stores only local paths and endpoint configuration.

3. patch
   - BillBot provides patch files or configuration snippets.
   - User applies them to their own local NapCatQQ installation.
   - Do not redistribute a modified NapCatQQ package.

4. bundled
   - Not recommended.
   - Only consider this with explicit permission from NapCatQQ's author and a clear license notice bundle.

BillBot release checklist
- Include LICENSE with LGPL-3.0-only text.
- Include SPDX headers in source files where practical.
- Include THIRD_PARTY_NOTICES.md for Go dependencies and external integrations.
- Do not include NapCatQQ archives, images, source snapshots, or patched builds.
- In dashboard setup, show that NapCatQQ is an external project with its own license.
- Require user confirmation before downloader mode installs NapCatQQ.
