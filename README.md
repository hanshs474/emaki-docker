# emaki CLI image

Official SDK for emaki (emaki.ai), the Japanese-language AI video generator for Seedance 2.5 and Seedance 2.0. Generate [Seedance 2.5](https://emaki.ai/seedance-2-5) and [Seedance 2.0](https://emaki.ai/seedance-2) clips, and images, from the command line (a Docker image of the emaki CLI) with the API key of your [emaki](https://emaki.ai/?utm_source=dockerhub&utm_medium=package) account.

```bash
docker run --rm -e EMAKI_API_KEY san8888/emaki "a paper lantern drifting down a night river, slow dolly-in"
# prints the URL of the finished MP4
```

Each run submits, waits for the result and prints the URL of the finished file. Every run costs credits on your emaki account, the same price as on the website.

## Install

```bash
docker pull san8888/emaki
```

A static Go binary on an empty base image, for `linux/amd64` and `linux/arm64`. Source: [github.com/hanshs474/emaki-docker](https://github.com/hanshs474/emaki-docker). Exit codes: 0 ok, 1 failure, 2 usage, 3 key problem, 4 out of credits or the model needs a purchase.

## API key

Sign up on [emaki.ai](https://emaki.ai/?utm_source=dockerhub&utm_medium=package), create a key at [emaki.ai/settings/apikeys](https://emaki.ai/settings/apikeys), then pass it or export it:

```bash
export EMAKI_API_KEY=sk-...
```

A key runs on your account exactly like the site does: same models, same credits, and every run shows up in your library on emaki.ai.

## Video

The default model is `seedance_2_5` (5 seconds, 480p, 16:9, with sound). Flags map to the request parameters; anything without a flag goes in `-params` as JSON:

```bash
# Seedance 2.5: 8 seconds, 720p, vertical
docker run --rm -e EMAKI_API_KEY san8888/emaki -duration 8 -resolution 720p -aspect 9:16 "a street drummer in the rain, handheld"

# Seedance 2.0
docker run --rm -e EMAKI_API_KEY san8888/emaki -model seedance_2 -duration 5 "a koi pond seen from above, petals falling"

# any other parameter as JSON
docker run --rm -e EMAKI_API_KEY san8888/emaki -params '{"generate_audio":false}' "a quiet library at dawn"
```

| model | what it is | main parameters |
|---|---|---|
| `seedance_2_5` | [Seedance 2.5](https://emaki.ai/seedance-2-5), the default | `duration` 4 to 30 · `resolution` 480p / 720p / 1080p · `aspect_ratio` 16:9, 4:3, 1:1, 3:4, 9:16, 21:9, adaptive · `generate_audio` · `image` · `last_frame_image` |
| `seedance_2` | [Seedance 2.0](https://emaki.ai/seedance-2) | `duration` 4 to 15 · `resolution` 480p / 720p / 1080p · `aspect_ratio` · `generate_audio` · `image` |
| `seedance_2_fast` | Seedance 2.0 Fast | `duration` 4 to 15 · `resolution` 480p / 720p · `aspect_ratio` · `image` |
| `seedance_2_mini` | Seedance 2.0 Mini, the cheapest video model | same as Fast |
| `minimax_h3` | MiniMax H3 | `duration` 5 to 15 · `aspect_ratio` · `image` |
| `veo_3_1` | Veo 3.1 | `resolution` 720p / 1080p / 4k · `aspectRatio` 16:9 / 9:16 · `image` |

### Image to video

Pass `image`, a public https URL, and it becomes the first frame; the prompt then describes the motion. This is the same [image to video](https://emaki.ai/image-to-video) flow as on the site:

```bash
docker run --rm -e EMAKI_API_KEY san8888/emaki -first-frame https://example.com/statue.jpg "the camera slowly orbits the statue"
```

On Seedance 2.5, `last_frame_image` pins the closing frame as well.

## Images

The default image model is `nano_banana_2_lite`, the cheapest run on the site:

```bash
docker run --rm -e EMAKI_API_KEY san8888/emaki -image "a ramen stall in the rain, neon reflections"
docker run --rm -e EMAKI_API_KEY san8888/emaki -image -model nano_banana_2 -aspect 16:9 "a ukiyo-e style wave over a modern city"
```

Image models: `nano_banana_2_lite`, `nano_banana_2` (`resolution` 1K / 2K / 4K), `seedream_5_lite` (`quality` basic / high / ultra), `gpt_image_2` (`resolution` 1K / 2K / 4K). All take `aspect_ratio`.

## Submit now, collect later

```bash
docker run --rm -e EMAKI_API_KEY san8888/emaki -no-wait "a fox trotting through fresh snow"   # prints the run id
docker run --rm -e EMAKI_API_KEY san8888/emaki -status <id>                                  # JSON: status, videoUrl, failureCode
docker run --rm -e EMAKI_API_KEY san8888/emaki -credits                                      # free to call
```

A run waits up to 15 minutes by default, polling every 5 seconds; change that with `-timeout 30m`.

## Errors worth catching

Errors print to stderr prefixed with their kind:

- `auth`: no key, or the key is wrong or deleted (HTTP 401)
- `quota`: not enough credits left for this run (HTTP 402)
- `plan`: this model unlocks after the account has bought credits (HTTP 403)
- `busy`: another generation on this account is still in progress (HTTP 409)
- `rejected`: the content filter refused the prompt, or the account is cooling down after a refusal; reword rather than retry
- `validation`: a parameter is missing or out of range; the message says which
- `failed`: the run was accepted and then failed; the failure code says why
- `timeout`: the wait ran out; the run may still finish in your emaki library

Anything else (a network failure, a 5xx) comes back as `other` with the service's message.

## More on emaki

Clips you make here can be sharpened afterwards with the [AI video upscaler](https://emaki.ai/video-upscaler) on the site.

日本語での使い方は [シーダンス](https://emaki.ai/) へ。

MIT licensed. Made by [emaki](https://emaki.ai/?utm_source=dockerhub&utm_medium=package).
