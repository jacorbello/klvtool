# Changelog

## 1.0.0 (2026-04-10)


### Features

* add backend environment checks ([5612c90](https://github.com/jacorbello/klvtool/commit/5612c90abbad48a5af8fe6869e56e541fe22dc46))
* add backend selection abstraction ([ff88f52](https://github.com/jacorbello/klvtool/commit/ff88f52a331b2c917841eef3d594cf19948b274a))
* add canonical extraction normalization ([9de5f0b](https://github.com/jacorbello/klvtool/commit/9de5f0ba8d27bc4b20cc43c0bbb30228eadbdc5f))
* add doctor command ([7765385](https://github.com/jacorbello/klvtool/commit/77653854f753b15d7f696711accf5d5cc6a44d50))
* add extraction manifest and error models ([7fb0f44](https://github.com/jacorbello/klvtool/commit/7fb0f44c9a1c29231eb2b9ad54f1ae102fcdfa08))
* add gstreamer discovery extraction ([f19d961](https://github.com/jacorbello/klvtool/commit/f19d9614ced9e5c3ead1759c607dc8e5338f9139))
* add make test-data target for sample video provisioning ([f3b58a9](https://github.com/jacorbello/klvtool/commit/f3b58a9ec9362a82f8ed447acb72696e464b8de7))
* add output writers ([ad4bdc2](https://github.com/jacorbello/klvtool/commit/ad4bdc2b883d6ca31c0b05fe1bf939824011244f))
* add versioned root command ([3aedf86](https://github.com/jacorbello/klvtool/commit/3aedf86ec5b48b73b937d1714dd7952fbf3e9d0c))
* enhance backend health checks and add extraction command ([fcab6ae](https://github.com/jacorbello/klvtool/commit/fcab6aee3136d98c2ebdd07a460514e20cc30180))
* native Go MPEG-TS parser (Layer 2) ([#8](https://github.com/jacorbello/klvtool/issues/8)) ([179661e](https://github.com/jacorbello/klvtool/commit/179661e47a2be534f803628e5f4615e0ea98e844))
* pretty doctor output with colors ([#4](https://github.com/jacorbello/klvtool/issues/4)) ([1faa9e6](https://github.com/jacorbello/klvtool/commit/1faa9e693477b33944313801e0f6abe106e13e8d))
* pretty doctor output with gstreamer detection fix ([#5](https://github.com/jacorbello/klvtool/issues/5)) ([9133bb4](https://github.com/jacorbello/klvtool/commit/9133bb4b0f1f6f0676c0682b04a7738a06626bfc))


### Bug Fixes

* add baseline help and diagnostics ([063f385](https://github.com/jacorbello/klvtool/commit/063f385ca8944735b73bd00c63112946b9ffeacf))
* address PR [#7](https://github.com/jacorbello/klvtool/issues/7) review feedback ([#7](https://github.com/jacorbello/klvtool/issues/7)) ([7efb169](https://github.com/jacorbello/klvtool/commit/7efb169b847ee3d08e957853a1f9d0936fd17fd3))
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
* harden backend selector normalization ([ced58fa](https://github.com/jacorbello/klvtool/commit/ced58fad9bc7013fd54475c0f74445e08f9b5b6a))
* harden envcheck guidance and health reporting ([a2a697c](https://github.com/jacorbello/klvtool/commit/a2a697c625ca15a3338e42a15b8249464d99df87))
* improve baseline cli usage behavior ([646ca21](https://github.com/jacorbello/klvtool/commit/646ca2156d50796ca6169d8b7e6f7b8614b7cbd5))
* preserve equal-pid record order ([085c7bf](https://github.com/jacorbello/klvtool/commit/085c7bf607eb6031209149d551a589fa245037b9))
* preserve ffmpeg parse warning detail ([7ece36d](https://github.com/jacorbello/klvtool/commit/7ece36d9b5759b7f6e41e8669ac3bbc34e5c3126))
* remove warnings from extract ordering ([44e2eb5](https://github.com/jacorbello/klvtool/commit/44e2eb5ca54b31e7eeebfc0ae0eea36d83564b04))
* rename baseline to klvtool ([137ae5d](https://github.com/jacorbello/klvtool/commit/137ae5dbda18706a3114001262660ceb8bf59177))
* stabilize manifest model serialization ([b8e0f93](https://github.com/jacorbello/klvtool/commit/b8e0f93a6b5ad3d36c84d4c36d772520e5d7fc19))
* tighten envcheck platform and probe behavior ([49a3516](https://github.com/jacorbello/klvtool/commit/49a3516d21a30b05839f98e114bf1cffca758560))
* use golangci-lint-action v7 for v2 lint ([1a77a5b](https://github.com/jacorbello/klvtool/commit/1a77a5bdb8fc70041d85846d8ea62670ffd32bfa))
* wire doctor output through root command ([acce6de](https://github.com/jacorbello/klvtool/commit/acce6de9fe3a66a73aaa53fd260d48d695218418))
