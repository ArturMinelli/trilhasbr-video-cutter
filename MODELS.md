# Detection models

TrilhasBR Video Cutter runs two ONNX models locally through ONNX Runtime. They are published as assets of the GitHub
release `models-v1` (https://github.com/ArturMinelli/trilhasbr-video-cutter/releases/tag/models-v1), are not stored in
the git history, and are verified by SHA-256 against `resources/models/manifest.json` before use. This project did not
train or fine-tune either model.

| | `person-yolo11n.onnx` | `osnet_x1_0_msmt17.onnx` |
|---|---|---|
| Role | Person detection: class 0 of the 80 COCO classes; the app keeps only `person` | Person re-identification: one 512-dimensional appearance embedding per person crop |
| Architecture | Ultralytics YOLO11n | OSNet x1.0 |
| Input | `1x3x1280x1280` RGB, letterboxed | `Bx3x256x128` RGB, ImageNet mean/std, B = batch (dynamic) |
| Output | `1x84x33600` (4 box values + 80 class scores per candidate) | `Bx512`, L2-normalized by the app |
| ONNX | opset 12, IR 7, PyTorch 2.8.0 | opset 17, IR 8, PyTorch 2.15.0 |
| Size | 11,245,601 bytes | 8,729,454 bytes |
| SHA-256 | `2c182881cc3d21614381e7c7b6d58feb1c6e49a509d14a67c6c5e2a34c62ee99` | `bad3a75234470f592b4c2b0a228b451347bba84e6181a6af7c5f93c4497cd564` |
| License | AGPL-3.0 (Ultralytics) | torchreid code MIT; training-data terms unconfirmed (research use) |

Shapes and hashes above are the ones in `resources/models/manifest.json`; the hashes were recomputed from the files.

## Provenance (what the files themselves say)

**YOLO11n.** The metadata the exporter embedded in the ONNX file reads: `description = Ultralytics YOLO11n model trained
on .../coco.yaml`, `author = Ultralytics`, `license = AGPL-3.0 License (https://ultralytics.com/license)`,
`version = 8.4.117`, `task = detect`, `imgsz = [1280, 1280]`, `batch = 1`, `stride = 32`,
`args = {data: None, dynamic: False, simplify: True, opset: 12, nms: False}`, exported 2026-09-22, and a `names` table
holding the 80 COCO classes. That is the stock Ultralytics COCO checkpoint `yolo11n.pt` exported to ONNX: not a
fine-tune, and not trained on this project's data.

Export command, **reconstructed from that metadata, not from a saved script**:

```
pip install ultralytics==8.4.117
yolo export model=yolo11n.pt format=onnx imgsz=1280 opset=12 simplify=True dynamic=False
mv yolo11n.onnx person-yolo11n.onnx
```

**OSNet.** Architecture and code: torchreid (https://github.com/KaiyangZhou/deep-person-reid, MIT). The weights are
OSNet x1.0 trained on MSMT17, in the family of the torchreid model zoo, converted to ONNX (opset 17, dynamic batch
axis, PyTorch 2.15.0 exporter). The ONNX file carries no metadata, and the original `.pth` checkpoint and the export
script are not in this repository, so the exact checkpoint (the Hugging Face repo https://huggingface.co/kaiyangzhou/osnet
lists `osnet_x1_0_msmt17_combineall_256x128_...` files) and the export commands are **not independently confirmed**.

## Licenses and what they mean for you

- **YOLO11n: AGPL-3.0.** Ultralytics states that its trained models fall under AGPL-3.0 by default and that using them
  in a product without open-sourcing the whole work requires an Ultralytics Enterprise License
  (https://ultralytics.com/license). This file is redistributed here under AGPL-3.0 with the Ultralytics notice kept in
  its metadata, and this repository is licensed AGPL-3.0-or-later for the same reason. If you build a closed or
  commercial product on it, read that page and consider an Enterprise license.
- **OSNet: research use until you have checked the dataset terms.** The torchreid code and its Hugging Face repo are
  MIT. The weights were trained on MSMT17, whose authors distribute the data under a signed release agreement
  (https://www.pkuvmc.com/dataset.html). We could not retrieve the agreement text, so we cannot say whether it limits
  commercial use of models trained on it. Check the agreement, or replace these weights with ones trained on data you
  may use commercially, before any commercial use.
- ONNX Runtime (MIT) and every other component: `THIRD_PARTY_LICENSES.md`.

This is a description of the licenses as we understand them, not legal advice.

## Intended use and limits

Detecting and tracking pedestrians in video footage on the user's own machine, to cut clips. Not for identifying named
individuals, and not evaluated for fairness across demographics, lighting or camera angles. Detection quality drops for
very small, occluded or heavily blurred people; boxes under about 64 px tall are not embedded. Measured on our own test
footage only: about 62 tracks across 7 phone videos, running 7 to 9 times faster than real time on an Apple M4.

## How the app gets them

`install.sh` and `scripts/build-mac.sh` run `scripts/fetch-models.sh`, which downloads each file from
`https://github.com/<GITHUB_REPO>/releases/download/<MODELS_TAG>/<file>` (`GITHUB_REPO` and `MODELS_TAG` live in
`scripts/release.env`; the tag is `models-v1`). Every file is checked against the pinned SHA-256 in
`resources/models/manifest.json`; a download that does not match is never kept. It retries 3 times, writes atomically,
skips files that are already valid and works offline when they are present. Set `MODELS_BASE_URL` to use a mirror or a
local `file:///some/dir` instead of the release.

Manual download:

```
gh release download models-v1 --repo ArturMinelli/trilhasbr-video-cutter --dir resources/models \
  --pattern '*.onnx' --pattern manifest.json
```

## Verify a download

```
cd resources/models
shasum -a 256 person-yolo11n.onnx osnet_x1_0_msmt17.onnx
```

The values must match the table above and `manifest.json`. The release also carries a `SHA256SUMS` file
(`shasum -a 256 -c SHA256SUMS` in the folder holding the downloaded files).
