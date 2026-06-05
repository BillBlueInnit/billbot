# Third Party Notices

BillBot thanks the open source and community projects that make this project possible.

BillBot itself is licensed under LGPL-3.0-only.

SPDX-License-Identifier: LGPL-3.0-only

Third party projects listed here are not relicensed by BillBot. Each project keeps its own license.

## Hermes Agent

Project: Hermes Agent
Repository: https://github.com/NousResearch/hermes-agent
License: MIT
Use in BillBot: optional external CLI runner
Thanks: Nous Research and Hermes Agent contributors
Distribution note: BillBot does not need to bundle Hermes. Users can install Hermes separately, or BillBot can detect an existing hermes command.

## NapCatQQ

Project: NapCatQQ
Repository: https://github.com/NapNeko/NapCatQQ
License: custom Limited Redistribution License, GitHub SPDX: NOASSERTION
Use in BillBot: optional QQ connector through OneBot-compatible HTTP/WebSocket endpoints
Thanks: NapCatQQ project authors and contributors
Distribution note: BillBot must not bundle NapCatQQ source code, binaries, patched builds, or images by default. Use external, installer, or patch mode instead.

## gorilla/websocket

Project: gorilla/websocket
Repository: https://github.com/gorilla/websocket
License: BSD-2-Clause
Use in BillBot: WebSocket connector support
Thanks: gorilla/websocket contributors

## gopkg.in/yaml.v3

Project: go-yaml
Repository: https://github.com/go-yaml/yaml
License: MIT and Apache-style notices depending on version files
Use in BillBot: YAML configuration loading and saving
Thanks: go-yaml contributors
