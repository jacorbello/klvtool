# Changelog

## [1.2.0](https://github.com/jacorbello/klvtool/compare/v1.1.2...v1.2.0) (2026-04-15)


### Features

* add diagnose command and shell completions ([#62](https://github.com/jacorbello/klvtool/issues/62)) ([a8f0c24](https://github.com/jacorbello/klvtool/commit/a8f0c248d2b65bda824ca22c0dc2a7ad1445b926))

## [1.1.2](https://github.com/jacorbello/klvtool/compare/v1.1.1...v1.1.2) (2026-04-15)


### Bug Fixes

* configure release-please manifest for draft releases ([#58](https://github.com/jacorbello/klvtool/issues/58)) ([ee39737](https://github.com/jacorbello/klvtool/commit/ee397371a226ccc4441523f68ebaaa14daf4e0cc))

## [1.1.1](https://github.com/jacorbello/klvtool/compare/v1.1.0...v1.1.1) (2026-04-15)


### Bug Fixes

* publish release assets via draft release ([#56](https://github.com/jacorbello/klvtool/issues/56)) ([9b5c4ed](https://github.com/jacorbello/klvtool/commit/9b5c4ed2217e8eb039a3d5fa97dc42a67076eb65))

## [1.1.0](https://github.com/jacorbello/klvtool/compare/v1.0.0...v1.1.0) (2026-04-14)


### Features

* add CSV output format to decode command ([#54](https://github.com/jacorbello/klvtool/issues/54)) ([0acf4e5](https://github.com/jacorbello/klvtool/commit/0acf4e52df0a47bf524da5323c53b522206b36b5))

## 1.0.0 (2026-04-14)


### Features

* add backend environment checks ([5612c90](https://github.com/jacorbello/klvtool/commit/5612c90abbad48a5af8fe6869e56e541fe22dc46))
* add backend selection abstraction ([ff88f52](https://github.com/jacorbello/klvtool/commit/ff88f52a331b2c917841eef3d594cf19948b274a))
* add canonical extraction normalization ([9de5f0b](https://github.com/jacorbello/klvtool/commit/9de5f0ba8d27bc4b20cc43c0bbb30228eadbdc5f))
* add doctor command ([7765385](https://github.com/jacorbello/klvtool/commit/77653854f753b15d7f696711accf5d5cc6a44d50))
* add extraction manifest and error models ([7fb0f44](https://github.com/jacorbello/klvtool/commit/7fb0f44c9a1c29231eb2b9ad54f1ae102fcdfa08))
* add gstreamer discovery extraction ([f19d961](https://github.com/jacorbello/klvtool/commit/f19d9614ced9e5c3ead1759c607dc8e5338f9139))
* add help as a root command alias for --help ([#27](https://github.com/jacorbello/klvtool/issues/27)) ([6911f5b](https://github.com/jacorbello/klvtool/commit/6911f5b6bfb17c7ec6450f5835b488fbdb5aff76))
* add make test-data target for sample video provisioning ([f3b58a9](https://github.com/jacorbello/klvtool/commit/f3b58a9ec9362a82f8ed447acb72696e464b8de7))
* add MISB ST 0601.19 KLV decoder (Layer 3 phase 1) ([#9](https://github.com/jacorbello/klvtool/issues/9)) ([1b7f019](https://github.com/jacorbello/klvtool/commit/1b7f0199b3d4049d4aa891aa197ac2a199bb0abc))
* add output writers ([ad4bdc2](https://github.com/jacorbello/klvtool/commit/ad4bdc2b883d6ca31c0b05fe1bf939824011244f))
* add versioned root command ([3aedf86](https://github.com/jacorbello/klvtool/commit/3aedf86ec5b48b73b937d1714dd7952fbf3e9d0c))
* complete ST 0601.19 tag coverage (all 143 tags) ([#12](https://github.com/jacorbello/klvtool/issues/12)) ([563542e](https://github.com/jacorbello/klvtool/commit/563542e0057d578ab4d80bc3523e438b60ea7fbd))
* display human-readable PTS alongside raw ticks in inspect ([#33](https://github.com/jacorbello/klvtool/issues/33)) ([b191475](https://github.com/jacorbello/klvtool/commit/b191475e280177fc7b7990c0802f063c395479a7))
* enhance backend health checks and add extraction command ([fcab6ae](https://github.com/jacorbello/klvtool/commit/fcab6aee3136d98c2ebdd07a460514e20cc30180))
* native Go MPEG-TS parser (Layer 2) ([#8](https://github.com/jacorbello/klvtool/issues/8)) ([179661e](https://github.com/jacorbello/klvtool/commit/179661e47a2be534f803628e5f4615e0ea98e844))
* pretty doctor output with colors ([#4](https://github.com/jacorbello/klvtool/issues/4)) ([1faa9e6](https://github.com/jacorbello/klvtool/commit/1faa9e693477b33944313801e0f6abe106e13e8d))
* pretty doctor output with gstreamer detection fix ([#5](https://github.com/jacorbello/klvtool/issues/5)) ([9133bb4](https://github.com/jacorbello/klvtool/commit/9133bb4b0f1f6f0676c0682b04a7738a06626bfc))
* warn when extract output directory already exists ([#30](https://github.com/jacorbello/klvtool/issues/30)) ([feff998](https://github.com/jacorbello/klvtool/commit/feff9989f4a23dd18df118ef8062fecae0715bad))
* warn when packetize output directory already exists ([#43](https://github.com/jacorbello/klvtool/issues/43)) ([a87d2fb](https://github.com/jacorbello/klvtool/commit/a87d2fbe609ab49987bd6991492cd5f0b3a3a2ec))


### Bug Fixes

* add baseline help and diagnostics ([063f385](https://github.com/jacorbello/klvtool/commit/063f385ca8944735b73bd00c63112946b9ffeacf))
* address PR [#7](https://github.com/jacorbello/klvtool/issues/7) review feedback ([#7](https://github.com/jacorbello/klvtool/issues/7)) ([7efb169](https://github.com/jacorbello/klvtool/commit/7efb169b847ee3d08e957853a1f9d0936fd17fd3))
* address QA findings across all CLI commands ([#52](https://github.com/jacorbello/klvtool/issues/52)) ([229a271](https://github.com/jacorbello/klvtool/commit/229a27134a3bdf93a46bf85051ef810acbad3c8c))
* align ffmpeg backend with canonical records ([d39083a](https://github.com/jacorbello/klvtool/commit/d39083a49a9a7d96e99512f2fcaa8c430fa6c2b0))
* align golangci-lint CI with v2 config ([676669c](https://github.com/jacorbello/klvtool/commit/676669c088ae25b97e175fd8e49547ebc6416948))
* align golangci-lint CI with v2 config ([c04ec1a](https://github.com/jacorbello/klvtool/commit/c04ec1a8848cd3b85f205a201bdee13d7d1dc43e))
* canonicalize equal-pid record ordering ([a817fa9](https://github.com/jacorbello/klvtool/commit/a817fa9cebbc9bc8a27787cdcffea8c45c7f731d))
* complete klvtool baseline rename ([7131931](https://github.com/jacorbello/klvtool/commit/7131931a20fa5dc56e0ef3d7acf0c295b54865a8))
* constrain wsl guidance by distro evidence ([523b4be](https://github.com/jacorbello/klvtool/commit/523b4be8f32c8222fa70f16bc52cb8067b95a64f))
* decouple extract selection from envcheck ([14a6eea](https://github.com/jacorbello/klvtool/commit/14a6eeaf2b9569051394c384e115d9b7f78ce7a9))
* derive distro guidance from os-release ([8c7bba1](https://github.com/jacorbello/klvtool/commit/8c7bba1d830b2e6810adb8d4879e9efa534f12be))
* fully deterministic extraction ordering ([1c36fd7](https://github.com/jacorbello/klvtool/commit/1c36fd780fbb2b6e8fefe731e9ac0c530e381f90))
* fully sync doctor writers from root ([160161f](https://github.com/jacorbello/klvtool/commit/160161fd16178efaa7fb3f1b7b8157b0f971a292))
* handle decode --out file close errors ([#29](https://github.com/jacorbello/klvtool/issues/29)) ([b7ff2ef](https://github.com/jacorbello/klvtool/commit/b7ff2efc87677c764e7814db60126016603f9571))
* handle flag.ErrHelp consistently across all commands ([#26](https://github.com/jacorbello/klvtool/issues/26)) ([86093fd](https://github.com/jacorbello/klvtool/commit/86093fd72ce17251716abcc24a46ddf047c4b198)), closes [#14](https://github.com/jacorbello/klvtool/issues/14)
* harden backend selector normalization ([ced58fa](https://github.com/jacorbello/klvtool/commit/ced58fad9bc7013fd54475c0f74445e08f9b5b6a))
* harden envcheck guidance and health reporting ([a2a697c](https://github.com/jacorbello/klvtool/commit/a2a697c625ca15a3338e42a15b8249464d99df87))
* improve baseline cli usage behavior ([646ca21](https://github.com/jacorbello/klvtool/commit/646ca2156d50796ca6169d8b7e6f7b8614b7cbd5))
* make doctor exit code reflect backend health ([#28](https://github.com/jacorbello/klvtool/issues/28)) ([2aacfb1](https://github.com/jacorbello/klvtool/commit/2aacfb1a6e1b74ea5a548fcc23b89f06593f8e0f))
* preserve equal-pid record order ([085c7bf](https://github.com/jacorbello/klvtool/commit/085c7bf607eb6031209149d551a589fa245037b9))
* preserve ffmpeg parse warning detail ([7ece36d](https://github.com/jacorbello/klvtool/commit/7ece36d9b5759b7f6e41e8669ac3bbc34e5c3126))
* reject stray positional arguments in version command ([#40](https://github.com/jacorbello/klvtool/issues/40)) ([ca9a800](https://github.com/jacorbello/klvtool/commit/ca9a800f1ae3dd301a380dfb8d67fabc5d7b3cc1)), closes [#38](https://github.com/jacorbello/klvtool/issues/38)
* remove warnings from extract ordering ([44e2eb5](https://github.com/jacorbello/klvtool/commit/44e2eb5ca54b31e7eeebfc0ae0eea36d83564b04))
* rename baseline to klvtool ([137ae5d](https://github.com/jacorbello/klvtool/commit/137ae5dbda18706a3114001262660ceb8bf59177))
* rename totalLength to valueLength in decode NDJSON schema ([#32](https://github.com/jacorbello/klvtool/issues/32)) ([54a4670](https://github.com/jacorbello/klvtool/commit/54a46708693bc78a18155f3a952631462010a9b9)), closes [#21](https://github.com/jacorbello/klvtool/issues/21)
* revert tag 13 Sensor Latitude from FormatIMAPB to FormatInt32 ([#11](https://github.com/jacorbello/klvtool/issues/11)) ([5868ed4](https://github.com/jacorbello/klvtool/commit/5868ed49d0e251bd18d986a41a077fd9510e4de4))
* stabilize manifest model serialization ([b8e0f93](https://github.com/jacorbello/klvtool/commit/b8e0f93a6b5ad3d36c84d4c36d772520e5d7fc19))
* suppress install guidance when all backends are healthy ([#34](https://github.com/jacorbello/klvtool/issues/34)) ([bbdd539](https://github.com/jacorbello/klvtool/commit/bbdd53941fc67b5d725f5afa1ee698ba70f356fe))
* surface manifest file close errors in extract and packetize ([#41](https://github.com/jacorbello/klvtool/issues/41)) ([e558515](https://github.com/jacorbello/klvtool/commit/e558515f899bfe77685a43c4c713b634e01be7b5)), closes [#36](https://github.com/jacorbello/klvtool/issues/36)
* tighten envcheck platform and probe behavior ([49a3516](https://github.com/jacorbello/klvtool/commit/49a3516d21a30b05839f98e114bf1cffca758560))
* use golangci-lint-action v7 for v2 lint ([1a77a5b](https://github.com/jacorbello/klvtool/commit/1a77a5bdb8fc70041d85846d8ea62670ffd32bfa))
* use model.Error consistently in version and packetize ([#48](https://github.com/jacorbello/klvtool/issues/48)) ([1ee1397](https://github.com/jacorbello/klvtool/commit/1ee1397f0072538e1fda306210120ffe8656dded))
* validate --pid flag range in decode command ([#25](https://github.com/jacorbello/klvtool/issues/25)) ([a5ed737](https://github.com/jacorbello/klvtool/commit/a5ed7373268ab1fdb8759b99c08a08c9bb58b1de))
* validate input directory existence in packetize command ([#42](https://github.com/jacorbello/klvtool/issues/42)) ([fd8c6ed](https://github.com/jacorbello/klvtool/commit/fd8c6ede4668d306c93694197f6c2b77db8fd51f))
* validate input file existence at CLI layer ([#31](https://github.com/jacorbello/klvtool/issues/31)) ([d08099a](https://github.com/jacorbello/klvtool/commit/d08099af5d9f24831670f4dcd6bd51501d8827cb))
* warn when PMT discovery hits packet cap without finding streams ([#35](https://github.com/jacorbello/klvtool/issues/35)) ([8b69f1c](https://github.com/jacorbello/klvtool/commit/8b69f1cc883f7781f24ad1ed9f63b6866d5bffbc)), closes [#23](https://github.com/jacorbello/klvtool/issues/23)
* wire doctor output through root command ([acce6de](https://github.com/jacorbello/klvtool/commit/acce6de9fe3a66a73aaa53fd260d48d695218418))
