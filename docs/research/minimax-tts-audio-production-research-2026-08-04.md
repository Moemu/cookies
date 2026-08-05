# MiniMax TTS 与品牌广告独立音轨生产调研

- 日期：2026-08-04
- 范围：MiniMax 官方 TTS 接入与 FFmpeg 官方音频编排、混音、成片封装能力
- 来源边界：只使用 MiniMax 与 FFmpeg 官方文档；每项外部事实在相邻段落标注来源
- 安全边界：本次未读取、记录或调用任何真实 API Key，也未修改业务代码

## 1. 结论

1. **现有 MiniMax Key 不一定需要更换，但必须做一次真实 capability probe。** MiniMax 的同步 TTS 使用标准 `Authorization: Bearer <API Key>`，入口为 `POST https://api.minimaxi.com/v1/t2a_v2`；官方 API 概览说明按量付费 API Key 可调用多模态能力，但仅凭当前数据库里存在一个 MiniMax 文本模型配置，无法确认这枚 Key 的类型、余额和 Speech 调用是否实际成功。[MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http) [MiniMax：接口概览](https://platform.minimaxi.com/docs/api-reference/api-overview)
2. **15 秒广告旁白优先走同步、非流式 TTS。** 同步接口文本上限小于 10,000 字符，超过 3,000 字符才建议流式；本项目的短广告旁白远低于该范围，因此无需引入长文本任务轮询。[MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http)
3. **MiniMax 返回的是供应商临时结果，不应直接成为 Cookies 的长期素材地址。** 同步非流式结果可返回 `hex` 或 URL，URL 有效期 24 小时；异步结果下载 URL 有效期 9 小时。因此成功后应立即解码或下载到项目自己的素材存储，并记录供应商、模型、音色与 `trace_id`。[MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http) [MiniMax：创建异步语音合成任务](https://platform.minimaxi.com/docs/api-reference/speech-t2a-async-create)
4. **FFmpeg 已覆盖独立音轨闭环所需的核心媒体原语。** 它可以对音频裁剪、时间偏移、多轨混合、旁白触发 BGM ducking、响度标准化，并通过显式 stream mapping 把原视频和最终混音封装为一个成片文件。[FFmpeg：Filters](https://ffmpeg.org/ffmpeg-filters.html) [FFmpeg：Stream selection / map](https://ffmpeg.org/ffmpeg.html#Stream-selection)
5. **第一版应保留真实 TTS 与 Fixture 双路由。** 这是基于上述 API 时效、鉴权和余额风险作出的工程建议：真实 provider probe 不通过时，仍用固定娇兰旁白资产跑通 AudioMixVersion、AudioTrack、AudioClip、MixJob 和最终 MP4 合成；不能把 Fixture 成功误报为 MiniMax 成功。

## 2. MiniMax TTS 官方接入事实

### 2.1 接口、鉴权与推荐调用形态

同步 HTTP 语音合成接口是 `POST /v1/t2a_v2`，主域名为 `https://api.minimaxi.com`，官方还列出 `https://api-bj.minimaxi.com/v1/t2a_v2` 作为备用地址。鉴权头为 `Authorization: Bearer <API Key>`，请求体使用 `application/json`。[MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http)

当前官方同步接口支持 `speech-2.8-hd`、`speech-2.8-turbo`、`speech-2.6-hd`、`speech-2.6-turbo` 以及更早的 Speech 01/02 系列。Speech 2.8 支持在文本中加入呼吸、轻笑等语气词标签；这类标签适合可选的表演增强，不应由系统无条件插入品牌旁白。[MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http)

同步接口既支持非流式也支持流式。非流式可用 `output_format=url|hex`，默认 `hex`；流式只返回 hex 形式。对本项目 15 秒旁白，建议使用 `stream=false`、`output_format=hex`，收到结果后直接解码并持久化，避免依赖 24 小时 URL。[MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http)

最小服务端探测请求可以采用：

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/t2a_v2 \
  --header "Authorization: Bearer $MINIMAX_API_KEY" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "speech-2.8-turbo",
    "text": "娇兰二十五倍蜂皇水。",
    "stream": false,
    "voice_setting": {
      "voice_id": "female-chengshu",
      "speed": 1,
      "vol": 1,
      "pitch": 0,
      "emotion": "calm"
    },
    "audio_setting": {
      "sample_rate": 32000,
      "bitrate": 128000,
      "format": "wav",
      "channel": 1
    },
    "subtitle_enable": true,
    "subtitle_type": "word",
    "output_format": "hex"
  }'
```

该请求形状来自官方同步接口；具体 `voice_id` 应先通过声音管理接口确认当前账号可见，不应把示例 ID 永久硬编码。[MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http) [MiniMax：查询可用音色 ID](https://platform.minimaxi.com/docs/api-reference/voice-management-get)

### 2.2 模型、音色与可编辑参数

`voice_setting.voice_id` 支持系统音色、复刻音色和文生音色。官方提供 `POST /v1/get_voice`，可按 `voice_type` 查询当前账号可用的系统、克隆和生成音色；因此 Cookies 应在服务端同步可用音色并映射为自己的稳定 `voice_alias`，而不是让前端直接依赖供应商 ID。[MiniMax：查询可用音色 ID](https://platform.minimaxi.com/docs/api-reference/voice-management-get)

官方同步接口允许控制语速、音量、音调和情绪：语速范围 `[0.5, 2]`，音量 `(0, 10]`，音调 `[-12, 12]`；情绪包括 `happy`、`sad`、`angry`、`fearful`、`disgusted`、`surprised`、`calm`、`fluent`、`whisper`，但不同模型对 `fluent/whisper` 的支持不同。[MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http)

同步接口目前可输出 `mp3`、`pcm`、`flac`、`wav`、`pcmu_raw`、`pcmu_wav` 和 `opus`；采样率可选 8,000 至 44,100 Hz 的官方枚举值，声道数为 1 或 2。工程上建议 TTS 中间资产优先保存 WAV/PCM，最终成片再编码 AAC，避免重复有损压缩；这是基于官方格式能力作出的工程选择，并非 MiniMax 的强制要求。[MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http)

`subtitle_enable=true` 可返回字幕/时间戳数据，粒度支持 `sentence`、`word` 和流式专用的 `word_streaming`。建议把词级时间戳作为 VoiceClip 的派生数据，用于字幕对齐、播放头高亮和“旁白是否塞进镜头时长”的自动校验。[MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http)

### 2.3 同步与异步边界

异步接口为 `POST /v1/t2a_async_v2`，随后通过 `GET /v1/query/t2a_async_query_v2?task_id=...` 查询。任务状态包括 Processing、Success、Failed 和 Expired，查询接口限制每秒最多 10 次。直接传 `text` 时最长 5 万字符；使用上传文件 ID 时单个 txt 小于 100 万字符。[MiniMax：创建异步语音合成任务](https://platform.minimaxi.com/docs/api-reference/speech-t2a-async-create) [MiniMax：查询语音生成任务状态](https://platform.minimaxi.com/docs/api-reference/speech-t2a-async-query)

该异步能力面向长文本，不是 15 秒品牌广告旁白的首选。但 provider seam 可以保留 `sync` 与 `async` 两种 job 类型，以便未来支持长口播、数字人或批量配音。

### 2.4 错误、权限与 capability probe

官方错误码中，`1004` 表示未授权、Token 不匹配或 Cookie 缺失，`2049` 表示无效 API Key，`1008` 表示余额不足，`1002` 表示请求频率超限，`2013` 表示参数错误。错误反馈应保存 `trace_id` 以便供应商排查，但不得记录 API Key 或完整 Authorization 头。[MiniMax：错误码查询](https://platform.minimaxi.com/docs/api-reference/errorcode)

建议把首次探测分为两步：

1. 调用 `POST /v1/get_voice` 验证鉴权并获得账号实际可用音色；
2. 使用最短中文旁白调用一次 `POST /v1/t2a_v2`，要求 `base_resp.status_code == 0`、`data` 非空、音频可解码且时长大于零。

第 2 步不可省略：声音列表成功只证明鉴权和查询能力，不能证明目标模型、余额、音色与实际 TTS 组合可用。若命中 `1004/2049/1008` 或音频校验失败，应把 provider 标记为 unavailable，并明确向用户展示“正在使用 Fixture 旁白”，不得静默伪装成真实生成。[MiniMax：查询可用音色 ID](https://platform.minimaxi.com/docs/api-reference/voice-management-get) [MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http) [MiniMax：错误码查询](https://platform.minimaxi.com/docs/api-reference/errorcode)

## 3. FFmpeg 官方能力映射

### 3.1 视频片段拼接

FFmpeg concat demuxer 可以按清单顺序虚拟拼接文件并调整时间戳，但所有输入必须具有相同的 streams、codec、time base 等；错误的输入 duration 会导致时间戳和拼接瑕疵。它适合项目当前已经标准化的 Seedance 视频单元。[FFmpeg：concat demuxer](https://ffmpeg.org/ffmpeg-formats.html#concat)

若片段需要同时重编码或存在音视频同步差异，可使用 concat filter。该滤镜要求每段具有相同数量的音视频流、各段从时间戳 0 开始；相应流的分辨率等参数仍需显式归一化。它会按每段最长流确定片段时长，并在必要时给较短音频补静音。[FFmpeg：concat filter](https://ffmpeg.org/ffmpeg-filters.html#concat)

工程建议：锁定生成单元时先标准化为统一分辨率、帧率、pixel format、音频采样率和 channel layout；满足同构条件时走 concat demuxer 快速 copy，不满足时走 concat filter 重编码。

### 3.2 音频裁剪与时间定位

`atrim` 保留输入音频的一段连续区间，支持 `start`、`end` 和 `duration` 等参数；它不会自动把时间戳重置为 0，官方建议需要从零开始时在后面接 `asetpts`。[FFmpeg：atrim](https://ffmpeg.org/ffmpeg-filters.html#atrim)

`adelay` 可按毫秒、秒或 sample 延迟一个或多个声道，延迟区间自动以静音填充。对于时间轴上的 `AudioClip.start_ms`，应使用 `adelay=<start_ms>:all=1`，避免只延迟单声道中的一个 channel。[FFmpeg：adelay](https://ffmpeg.org/ffmpeg-filters.html#adelay)

### 3.3 多轨混音

`amix` 把多个音频输入混合为一个输出，支持 `inputs`、`duration`、`weights`、`normalize` 和输入结束时的 `dropout_transition`。它只处理浮点 sample；整数输入会自动插入 `aresample` 转换。`duration` 可选 longest、shortest 或 first，默认 longest。[FFmpeg：amix](https://ffmpeg.org/ffmpeg-filters.html#amix)

对 Voice、BGM、SFX 和可选 OriginalSound 轨道，推荐先分别完成 trim、timestamp reset、delay、fade/volume，再送入 `amix`。每条轨道的用户音量应转换成可审计的线性 gain 或 dB 参数，并在 AudioMixVersion 中保存原始值。

### 3.4 旁白触发 BGM 自动避让（ducking）

`sidechaincompress` 接收两个输入、输出一个流：第一个输入是被处理信号，第二个输入是检测/触发信号；第二路超过阈值时会压缩第一路。将 BGM 作为第一输入、旁白作为第二输入，即可在有人声时自动压低音乐，并通过 threshold、ratio、attack、release 等参数控制避让强度和恢复速度。[FFmpeg：sidechaincompress](https://ffmpeg.org/ffmpeg-filters.html#sidechaincompress)

这比简单把全片 BGM 固定调低更适合品牌广告：无人声区间仍可保持音乐能量，有旁白时自动让位。产品层只需暴露“轻微 / 标准 / 明显”三个避让预设，高级参数留在服务端 preset 中。

### 3.5 响度标准化

`loudnorm` 实现 EBU R128 响度标准化，支持 single-pass 和 double-pass，并可设定 integrated loudness、loudness range 和 maximum true peak。文件型最终交付建议使用 double-pass：第一遍测量，第二遍带 measured 参数做确定性归一化；低延迟预览可用 single-pass。[FFmpeg：loudnorm](https://ffmpeg.org/ffmpeg-filters.html#loudnorm)

具体交付目标值应由 Cookies 的平台规范或投放渠道规范确定；FFmpeg 官方只提供目标参数能力，不替项目决定广告平台的最终 LUFS 标准。

### 3.6 音视频封装与结束时长

FFmpeg 的 `-map` 可显式选择进入输出文件的视频和音频流；只要设置了 map，就不会再依赖自动 stream selection。输出选项 `-shortest` 会在最短输出流结束时停止 mux，因此最终混音必须预先 pad/trim 到 master timeline 时长，否则可能提前截断成片。[FFmpeg：Stream selection / map](https://ffmpeg.org/ffmpeg.html#Stream-selection) [FFmpeg：Formats shortest](https://ffmpeg.org/ffmpeg-formats.html)

推荐明确映射 `-map 0:v:0 -map "[mix]"`，视频未经过滤时可以 copy，最终混音编码为 MP4 广泛支持的 AAC；是否可直接 copy 仍取决于源视频 codec 与目标容器兼容性，不能全局硬编码。

## 4. 建议的 15 秒合成滤镜图

以下命令只表达滤镜拓扑，参数需在实现阶段结合实际输入流、FFmpeg build 和投放规范验证：

```bash
ffmpeg \
  -i locked-video.mp4 \
  -i voice.wav \
  -i bgm.wav \
  -i water-drop.wav \
  -filter_complex "\
    [1:a]atrim=duration=15,asetpts=PTS-STARTPTS,adelay=800:all=1,volume=1.0[voice];\
    [2:a]atrim=duration=15,asetpts=PTS-STARTPTS,volume=0.22[bgm];\
    [bgm][voice]sidechaincompress=threshold=0.03:ratio=8:attack=20:release=300[ducked_bgm];\
    [3:a]atrim=duration=1.2,asetpts=PTS-STARTPTS,adelay=4800:all=1,volume=0.7[sfx];\
    [voice][ducked_bgm][sfx]amix=inputs=3:duration=longest:normalize=0,\
      loudnorm=I=-16:LRA=7:TP=-1.5[mix]" \
  -map 0:v:0 -map "[mix]" \
  -c:v copy -c:a aac -b:a 192k -shortest final.mp4
```

此拓扑对应：旁白定位到 0.8 秒、音效定位到 4.8 秒、旁白驱动 BGM ducking、三轨合并、最终响度处理，再与锁定画面 mux。`atrim`、`asetpts`、`adelay`、`sidechaincompress`、`amix`、`loudnorm` 与 `-map/-shortest` 的行为均来自上述 FFmpeg 官方文档。

生产实现还应增加：对每个输入做 `ffprobe` 校验、统一采样率/channel layout、master duration 的 pad/trim、命令参数数组化而非字符串拼接、临时目录隔离、超时与磁盘配额、输出再次 `ffprobe` 验收。

## 5. 对 Cookies 技术设计的直接约束

这些是基于官方能力作出的项目设计建议，不是供应商接口的强制对象名：

- `VoiceProvider`：`ListVoices`、`Synthesize`、`ProbeCapability`；MiniMax 只是一个 adapter，Fixture 是另一个 adapter。
- `VoiceAsset`：保存 Cookies 自有对象存储地址、时长、format、sample rate、channel、checksum、provider/model/voice、`trace_id` 与时间戳 cues；禁止长期保存供应商临时 URL 作为唯一地址。
- `AudioClip`：保存引用资产、`start_ms/end_ms/source_in_ms/source_out_ms`、gain、fade、mute、来源镜头和旁白修订号。
- `AudioTrack`：区分 voice、music、sfx、original_sound；轨道和 clip 都可静音，但最终渲染由不可变 AudioMixVersion 快照驱动。
- `MixJob`：区分 preview 与 final；preview 可单遍响度处理和较低码率，final 使用确定性参数并保存 ffprobe 验收结果。
- `FallbackDisclosure`：MiniMax probe 失败时显式展示 `provider=fixture` 和失败分类，禁止在 UI 上显示“MiniMax 已生成”。

## 6. 仍不确定、必须在开发阶段验证的事项

1. 当前数据库中 MiniMax Key 是按量付费 Key 还是 Token Plan Key，以及它对 `speech-2.8-hd/turbo` 的实际可用性、余额、速率限制和计费；本次没有调用真实凭据。
2. 当前账号通过 `/v1/get_voice` 实际返回哪些中文女声；官方系统音色目录会变化，必须运行时查询或定期同步。
3. Speech 2.8 对娇兰产品名、数字“25X/二十五倍”和品牌外文名的实际发音质量，需要用 pronunciation dictionary 做样本验收。
4. 当前部署镜像中的 FFmpeg build 是否启用 `loudnorm`、`sidechaincompress` 及目标 AAC encoder；应以 `ffmpeg -filters`、`ffmpeg -encoders` 和一条最小合成测试确认。
5. 抖音等实际投放渠道要求的最终响度、true peak、音频 codec/bitrate 与版权审计字段；FFmpeg 能实现参数，但业务目标值需要渠道规范和产品决策。

## 7. 一手来源索引

- [MiniMax：接口概览](https://platform.minimaxi.com/docs/api-reference/api-overview)
- [MiniMax：同步语音合成 HTTP](https://platform.minimaxi.com/docs/api-reference/speech-t2a-http)
- [MiniMax：创建异步语音合成任务](https://platform.minimaxi.com/docs/api-reference/speech-t2a-async-create)
- [MiniMax：查询异步语音任务状态](https://platform.minimaxi.com/docs/api-reference/speech-t2a-async-query)
- [MiniMax：查询可用音色 ID](https://platform.minimaxi.com/docs/api-reference/voice-management-get)
- [MiniMax：系统音色列表](https://platform.minimaxi.com/docs/faq/system-voice-id)
- [MiniMax：错误码查询](https://platform.minimaxi.com/docs/api-reference/errorcode)
- [FFmpeg：Filters Documentation](https://ffmpeg.org/ffmpeg-filters.html)
- [FFmpeg：Formats Documentation](https://ffmpeg.org/ffmpeg-formats.html)
- [FFmpeg：ffmpeg Documentation](https://ffmpeg.org/ffmpeg.html)
