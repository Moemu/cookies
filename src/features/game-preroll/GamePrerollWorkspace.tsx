import { useEffect, useMemo, useRef, useState } from "react";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  CircleAlert,
  Clock3,
  Download,
  FileVideo,
  Gamepad2,
  Info,
  LoaderCircle,
  LockKeyhole,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  ShieldCheck,
  Sparkles,
  Trash2,
  Upload,
  WandSparkles,
} from "lucide-react";
import { useProject } from "../../context/ProjectContext";
import { MockGamePrerollClient } from "./mockClient";
import {
  briefCompleteness,
  createInitialGamePrerollState,
  generationBlockers,
  stepAccessible,
  stepOrder,
  type BriefField,
  type GamePrerollState,
  type GamePrerollStep,
  type HookCandidate,
  type Provenance,
} from "./model";
import "./gamePreroll.css";

const client = new MockGamePrerollClient();
const stepLabels: Record<GamePrerollStep, string> = {
  upload: "上传原视频",
  analysis: "AI 素材拆解",
  brief: "确认广告简报",
  candidates: "选择钩子方案",
  generate: "生成前贴",
};
const provenanceLabels: Record<Provenance, string> = {
  video_evidence: "视频证据",
  ai_inference: "AI 推断",
  manual: "人工补充",
};

function storageKey(projectId: string) {
  return `cookies:game-preroll:frontend-v2:${projectId}`;
}

function restore(projectId: string) {
  try {
    const saved = window.sessionStorage.getItem(storageKey(projectId));
    if (!saved) return createInitialGamePrerollState(projectId);
    const state = JSON.parse(saved) as GamePrerollState;
    return {
      ...state,
      source: state.source
        ? { ...state.source, previewUrl: undefined }
        : undefined,
    };
  } catch {
    return createInitialGamePrerollState(projectId);
  }
}

export function GamePrerollWorkspace({
  onNotice,
}: {
  onNotice: (message: string) => void;
}) {
  const { currentProject } = useProject();
  const [state, setState] = useState(() => restore(currentProject.id));
  const [busy, setBusy] = useState("");
  const fileInput = useRef<HTMLInputElement>(null);
  const sourceUrl = state.source?.previewUrl;

  useEffect(() => {
    setState(restore(currentProject.id));
  }, [currentProject.id]);
  useEffect(() => {
    const serializable = {
      ...state,
      source: state.source
        ? { ...state.source, previewUrl: undefined }
        : undefined,
    };
    window.sessionStorage.setItem(
      storageKey(currentProject.id),
      JSON.stringify(serializable),
    );
  }, [currentProject.id, state]);

  const selectedCandidate = useMemo(
    () =>
      state.candidates.find(
        (candidate) => candidate.id === state.selectedCandidateId,
      ),
    [state.candidates, state.selectedCandidateId],
  );

  const update = (patch: Partial<GamePrerollState>) =>
    setState((current) => ({ ...current, ...patch }));
  const go = (step: GamePrerollStep) => {
    if (!stepAccessible(state, step)) return;
    update({ step });
  };

  const upload = async (file?: File) => {
    if (!file) return;
    if (!file.type.startsWith("video/")) {
      update({ notice: "请选择 MP4、MOV 或其他常见视频格式。" });
      return;
    }
    setBusy("upload");
    try {
      const previewUrl = URL.createObjectURL(file);
      const source = await client.upload(file, previewUrl);
      update({
        source,
        step: "analysis",
        analysisStatus: "running",
        notice: "原视频已上传，正在进行 AI 素材拆解。",
      });
      const result = await client.analyze(source);
      setState((current) => ({
        ...current,
        analysisStatus: "succeeded",
        analysisFacts: result.facts,
        evidence: result.evidence,
        brief: result.brief,
        notice: "素材拆解完成，所有结论均已标注来源。",
      }));
      onNotice("游戏原视频已完成前端模拟拆解。");
    } catch (cause) {
      update({
        analysisStatus: "failed",
        notice:
          cause instanceof Error ? cause.message : "素材拆解失败，请重试。",
      });
    } finally {
      setBusy("");
    }
  };

  const confirmAnalysis = () =>
    update({
      step: "brief",
      notice: "请确认 AI 预填的广告简报；所有字段均可编辑。",
    });
  const confirmBrief = async () => {
    if (briefCompleteness(state.brief) < 100) {
      update({ notice: "请先填写全部必填字段。" });
      return;
    }
    setBusy("plan");
    try {
      const candidates = await client.plan(
        state.brief,
        state.evidence,
        state.config.durationSeconds,
      );
      setState((current) => ({
        ...current,
        briefConfirmed: true,
        candidates,
        selectedCandidateId: undefined,
        step: "candidates",
        notice: "已生成 3 套钩子规划；它们尚未调用视频模型。",
      }));
    } finally {
      setBusy("");
    }
  };

  const regenerate = async () => {
    setBusy("plan");
    try {
      const candidates = await client.plan(
        state.brief,
        state.evidence,
        state.config.durationSeconds,
      );
      update({
        candidates,
        selectedCandidateId: undefined,
        notice: "已重新规划 3 套候选，请再次人工选择。",
      });
    } finally {
      setBusy("");
    }
  };

  const generate = async () => {
    if (!selectedCandidate || generationBlockers(state).length) return;
    setBusy("generate");
    update({
      generation: { id: "submitting", status: "running", progress: 24 },
      notice: "正在创建 Seedance 生成任务（当前为前端模拟）。",
    });
    try {
      const generation = await client.generate(selectedCandidate, state.config);
      update({
        generation,
        notice: generation.diagnostic || "游戏前贴生成完成。",
      });
      onNotice("游戏前贴前端模拟生成完成。");
    } catch (cause) {
      update({
        generation: {
          id: "failed",
          status: "failed",
          progress: 0,
          diagnostic: cause instanceof Error ? cause.message : "生成失败",
        },
        notice: "生成失败，请保留当前配置后重试。",
      });
    } finally {
      setBusy("");
    }
  };

  return (
    <section className="gp-workspace" aria-label="游戏前贴五步创作工作区">
      <header className="gp-header">
        <div>
          <h2>游戏前贴</h2>
          <p>上传真实游戏素材，由 AI 拆解玩法并规划高注意力开场。</p>
        </div>
        <span>仅生成前贴 · 不拼接 · 不投放</span>
      </header>
      <StepRail state={state} onStep={go} />
      <div className="gp-body">
        {state.step === "upload" ? (
          <UploadStep
            state={state}
            busy={busy === "upload"}
            fileInput={fileInput}
            onUpload={upload}
            onDuration={(durationSeconds) =>
              update({ config: { ...state.config, durationSeconds } })
            }
          />
        ) : null}
        {state.step === "analysis" ? (
          <AnalysisStep
            state={state}
            sourceUrl={sourceUrl}
            busy={busy === "upload"}
            onRetry={() =>
              state.source &&
              void upload(
                new File([], state.source.name, { type: "video/mp4" }),
              )
            }
            onConfirm={confirmAnalysis}
          />
        ) : null}
        {state.step === "brief" ? (
          <BriefStep
            state={state}
            busy={busy === "plan"}
            onChange={(brief) =>
              update({
                brief,
                briefConfirmed: false,
                candidates: [],
                selectedCandidateId: undefined,
              })
            }
            onConfirm={() => void confirmBrief()}
          />
        ) : null}
        {state.step === "candidates" ? (
          <CandidateStep
            state={state}
            busy={busy === "plan"}
            onSelect={(selectedCandidateId) => update({ selectedCandidateId })}
            onRegenerate={() => void regenerate()}
            onNext={() => go("generate")}
          />
        ) : null}
        {state.step === "generate" ? (
          <GenerateStep
            state={state}
            candidate={selectedCandidate}
            busy={busy === "generate"}
            onConfig={(config) => update({ config })}
            onGenerate={() => void generate()}
            onDownload={() =>
              update({
                notice:
                  "下载入口已完成前端演示；接入后端后将下载正式 Project Asset。",
              })
            }
            onBack={() => go("candidates")}
          />
        ) : null}
      </div>
      <footer className="gp-footer">
        <div className="gp-notice" role="status">
          {busy ? (
            <LoaderCircle className="gp-spin" size={15} />
          ) : (
            <Info size={15} />
          )}
          <span>{state.notice}</span>
        </div>
        <small>前端 Mock 模式 · 后端接口将在下一阶段接入</small>
      </footer>
    </section>
  );
}

function StepRail({
  state,
  onStep,
}: {
  state: GamePrerollState;
  onStep: (step: GamePrerollStep) => void;
}) {
  const currentIndex = stepOrder.indexOf(state.step);
  return (
    <nav className="gp-steps" aria-label="游戏前贴创作步骤">
      <ol>
        {stepOrder.map((step, index) => {
          const accessible = stepAccessible(state, step);
          return (
            <li
              key={step}
              className={
                index === currentIndex
                  ? "active"
                  : index < currentIndex
                    ? "done"
                    : ""
              }
            >
              <button
                type="button"
                disabled={!accessible}
                aria-current={step === state.step ? "step" : undefined}
                onClick={() => onStep(step)}
              >
                <i>{index < currentIndex ? <Check size={13} /> : index + 1}</i>
                <span>{stepLabels[step]}</span>
              </button>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

function UploadStep({
  state,
  busy,
  fileInput,
  onUpload,
  onDuration,
}: {
  state: GamePrerollState;
  busy: boolean;
  fileInput: React.RefObject<HTMLInputElement | null>;
  onUpload: (file?: File) => void;
  onDuration: (duration: 6 | 7 | 8 | 9 | 10) => void;
}) {
  return (
    <div className="gp-layout gp-upload-layout">
      <main className="gp-panel gp-upload-panel">
        <SectionTitle
          index="01"
          title="上传游戏原视频"
          description="上传包含真实玩法、UI、操作结果或奖励反馈的视频。"
        />
        <button
          className="gp-dropzone"
          type="button"
          onClick={() => fileInput.current?.click()}
        >
          <Upload size={36} />
          <b>{busy ? "正在上传并建立素材…" : "点击上传或拖拽文件到此处"}</b>
          <span>支持 MP4 / MOV / AVI / MKV，建议包含完整“操作 → 结果”</span>
          <em>
            <FileVideo size={16} />
            选择游戏视频
          </em>
          <input
            ref={fileInput}
            type="file"
            accept="video/*"
            onChange={(event) => onUpload(event.target.files?.[0])}
          />
        </button>
        <div className="gp-guidance">
          <Info size={17} />
          <div>
            <b>什么样的视频更容易生成好前贴？</b>
            <p>
              尽量包含一次完整的操作和结果，例如选技能、挑战失败、清屏或领奖励，无需提前剪辑。
            </p>
          </div>
        </div>
        <div className="gp-requirements">
          <b>文件要求</b>
          <span>
            <Check size={14} /> 建议 720p 以上
          </span>
          <span>
            <Check size={14} /> 建议 15 秒以上
          </span>
          <span>
            <Check size={14} /> 单文件不超过 2 GB
          </span>
          <span>
            <Check size={14} /> 请确保素材已获授权
          </span>
        </div>
      </main>
      <aside className="gp-panel gp-preset">
        <SectionTitle title="输出预设" description="第一版面向抖音竖屏广告" />
        <Field label="投放平台">
          <input value="抖音" readOnly />
        </Field>
        <Field label="视频比例">
          <input value="9:16 竖屏" readOnly />
        </Field>
        <Field label={`前贴时长 · ${state.config.durationSeconds} 秒`}>
          <input
            type="range"
            min="6"
            max="10"
            step="1"
            value={state.config.durationSeconds}
            onChange={(event) =>
              onDuration(Number(event.target.value) as 6 | 7 | 8 | 9 | 10)
            }
          />
          <span className="gp-range-label">
            <i>6秒</i>
            <i>8秒 推荐</i>
            <i>10秒</i>
          </span>
        </Field>
        <div className="gp-scope-note">
          <Check size={17} />
          <div>
            <b>仅生成前贴，不拼接、不投放</b>
            <p>后续能力不会混入本次前端闭环。</p>
          </div>
        </div>
        <button
          className="gp-primary"
          type="button"
          disabled={busy}
          onClick={() => fileInput.current?.click()}
        >
          {busy ? <LoaderCircle className="gp-spin" /> : <Upload />}
          上传并开始拆解
        </button>
      </aside>
    </div>
  );
}

function AnalysisStep({
  state,
  sourceUrl,
  busy,
  onRetry,
  onConfirm,
}: {
  state: GamePrerollState;
  sourceUrl?: string;
  busy: boolean;
  onRetry: () => void;
  onConfirm: () => void;
}) {
  return (
    <div className="gp-layout gp-analysis-layout">
      <SourcePanel state={state} sourceUrl={sourceUrl} />
      <main className="gp-panel gp-analysis">
        <SectionTitle
          index="02"
          title="AI 素材拆解"
          description="识别真实玩法与证据，再形成可编辑的广告理解。"
        />
        <div className={`gp-run-status ${state.analysisStatus}`}>
          {state.analysisStatus === "running" ? (
            <LoaderCircle className="gp-spin" />
          ) : state.analysisStatus === "failed" ? (
            <CircleAlert />
          ) : (
            <Sparkles />
          )}
          <b>
            {state.analysisStatus === "running"
              ? "正在理解视频画面与操作结果…"
              : state.analysisStatus === "failed"
                ? "素材拆解失败"
                : `拆解完成 · 发现 ${state.evidence.length} 个高价值片段`}
          </b>
        </div>
        {state.analysisStatus === "succeeded" ? (
          <>
            <div className="gp-facts">
              {state.analysisFacts.map((fact) => (
                <article key={fact.id}>
                  <span>{fact.label}</span>
                  <b>{fact.value}</b>
                  <ProvenanceBadge value={fact.provenance} />
                </article>
              ))}
            </div>
            <div className="gp-truth-boundary">
              <ShieldCheck size={22} />
              <div>
                <b>真实性边界已建立</b>
                <p>
                  禁止生成未在视频中出现的奖励、数值和游戏
                  UI；允许放大真实操作反馈与字幕包装。
                </p>
              </div>
            </div>
          </>
        ) : (
          <div className="gp-analysis-skeleton">
            {Array.from({ length: 6 }, (_, index) => (
              <i key={index} />
            ))}
          </div>
        )}
        <div className="gp-actions">
          <button
            className="gp-secondary"
            type="button"
            disabled={busy}
            onClick={onRetry}
          >
            <RefreshCw />
            重新拆解
          </button>
          <button
            className="gp-primary"
            type="button"
            disabled={state.analysisStatus !== "succeeded"}
            onClick={onConfirm}
          >
            确认拆解结果
            <ArrowRight />
          </button>
        </div>
      </main>
    </div>
  );
}

function SourcePanel({
  state,
  sourceUrl,
}: {
  state: GamePrerollState;
  sourceUrl?: string;
}) {
  return (
    <aside className="gp-panel gp-source">
      <SectionTitle
        title="上传的原视频"
        description={
          state.source
            ? `${state.source.name} · ${Math.round(state.source.sizeBytes / 1024 / 1024)} MB`
            : "等待上传"
        }
      />
      <div className="gp-source-video">
        {sourceUrl ? (
          <video src={sourceUrl} controls muted />
        ) : (
          <div>
            <Gamepad2 />
            <b>游戏画面预览</b>
            <span>9:16 · 00:42</span>
          </div>
        )}
      </div>
      <h3>视频证据片段</h3>
      <div className="gp-evidence-list">
        {state.evidence.map((moment, index) => (
          <button type="button" key={moment.id}>
            <span className={`gp-evidence-thumb gp-scene-${index + 1}`}>
              <Play size={13} />
            </span>
            <span>
              <b>{moment.label}</b>
              <small>{formatRange(moment.startMs, moment.endMs)}</small>
            </span>
            <ProvenanceBadge value={moment.provenance} />
          </button>
        ))}
      </div>
    </aside>
  );
}

function BriefStep({
  state,
  busy,
  onChange,
  onConfirm,
}: {
  state: GamePrerollState;
  busy: boolean;
  onChange: (brief: BriefField[]) => void;
  onConfirm: () => void;
}) {
  const completeness = briefCompleteness(state.brief);
  const updateField = (id: string, value: string) =>
    onChange(
      state.brief.map((field) =>
        field.id === id ? { ...field, value } : field,
      ),
    );
  const removeField = (id: string) =>
    onChange(state.brief.filter((field) => field.id !== id));
  const addField = () =>
    onChange([
      ...state.brief,
      {
        id: `manual-${Date.now()}`,
        label: "补充信息",
        value: "",
        provenance: "manual",
        required: false,
        evidenceRefs: [],
      },
    ]);
  return (
    <div className="gp-layout gp-brief-layout">
      <main className="gp-panel gp-brief">
        <SectionTitle
          index="03"
          title="确认广告简报"
          description="AI 从视频中预填；你可以编辑、增加或删除，确认后才用于创意规划。"
        />
        <div className="gp-provenance-legend">
          字段来源：
          <ProvenanceBadge value="video_evidence" />
          <ProvenanceBadge value="ai_inference" />
          <ProvenanceBadge value="manual" />
        </div>
        <div className="gp-brief-table">
          <header>
            <span>字段</span>
            <span>信息来源</span>
            <span>内容（可编辑）</span>
            <span>操作</span>
          </header>
          {state.brief.map((field) => (
            <div className="gp-brief-row" key={field.id}>
              <b>
                {field.label}
                {field.required ? <sup>*</sup> : null}
              </b>
              <ProvenanceBadge value={field.provenance} />
              <textarea
                value={field.value}
                onChange={(event) => updateField(field.id, event.target.value)}
                aria-label={field.label}
              />
              <button
                type="button"
                aria-label={`删除${field.label}`}
                onClick={() => removeField(field.id)}
              >
                <Trash2 size={17} />
              </button>
            </div>
          ))}
        </div>
        <button className="gp-add-field" type="button" onClick={addField}>
          <Plus />
          新增字段
        </button>
        <div className="gp-actions">
          <span className="gp-autosave">
            <Check size={15} />
            本地草稿已自动保存
          </span>
          <button
            className="gp-primary"
            type="button"
            disabled={busy || completeness < 100}
            onClick={onConfirm}
          >
            {busy ? <LoaderCircle className="gp-spin" /> : <Sparkles />}确认
            Brief 并生成 3 个候选
          </button>
        </div>
      </main>
      <aside className="gp-panel gp-readiness">
        <SectionTitle title="生成前检查" description="候选规划所需信息" />
        {[
          "必填字段已填写",
          "信息来源已标注",
          "卖点有视频证据",
          "CTA 已明确",
        ].map((item, index) => (
          <p
            key={item}
            className={index === 0 && completeness < 100 ? "pending" : ""}
          >
            {index === 0 && completeness < 100 ? <CircleAlert /> : <Check />}
            <span>
              <b>{item}</b>
              <small>
                {index === 2 ? "不使用未经确认的奖励和数值" : "已满足"}
              </small>
            </span>
          </p>
        ))}
        <div className="gp-completeness">
          <span>
            Brief 完整度<b>{completeness}%</b>
          </span>
          <i>
            <em style={{ width: `${completeness}%` }} />
          </i>
        </div>
        <div className="gp-info-card">
          <Info />
          <p>
            不需要编写 Prompt。AI 会把确认后的 Brief 转成三种差异化钩子规划。
          </p>
        </div>
      </aside>
    </div>
  );
}

function CandidateStep({
  state,
  busy,
  onSelect,
  onRegenerate,
  onNext,
}: {
  state: GamePrerollState;
  busy: boolean;
  onSelect: (id: string) => void;
  onRegenerate: () => void;
  onNext: () => void;
}) {
  const selected = state.candidates.find(
    (candidate) => candidate.id === state.selectedCandidateId,
  );
  return (
    <main className="gp-panel gp-candidates">
      <div className="gp-candidate-title">
        <SectionTitle
          index="04"
          title="选择钩子方案"
          description="三套方案共享同一事实边界，但前三秒的吸引方式不同。先选创意，再生成视频。"
        />
        <button
          className="gp-secondary"
          type="button"
          disabled={busy}
          onClick={onRegenerate}
        >
          {busy ? <LoaderCircle className="gp-spin" /> : <RefreshCw />}重新规划
          3 套方案
        </button>
      </div>
      <div className="gp-candidate-grid">
        {state.candidates.map((candidate, index) => (
          <CandidateCard
            key={candidate.id}
            candidate={candidate}
            index={index}
            selected={candidate.id === state.selectedCandidateId}
            onSelect={() => onSelect(candidate.id)}
          />
        ))}
      </div>
      <div className="gp-recommendation">
        <Sparkles />
        <div>
          <b>{selected ? `已选择“${selected.name}”` : "AI 推荐“冲突反转”"}</b>
          <p>
            {selected?.recommendation ||
              state.candidates.find((item) => item.recommended)?.recommendation}
          </p>
          <small>推荐仅代表证据匹配度，不是 CTR 或转化率预测。</small>
        </div>
        <button
          className="gp-primary"
          type="button"
          disabled={!selected}
          onClick={onNext}
        >
          使用所选方案
          <ArrowRight />
        </button>
      </div>
    </main>
  );
}

function CandidateCard({
  candidate,
  index,
  selected,
  onSelect,
}: {
  candidate: HookCandidate;
  index: number;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <article className={`gp-candidate ${selected ? "selected" : ""}`}>
      <button
        className="gp-candidate-select"
        type="button"
        onClick={onSelect}
        aria-pressed={selected}
      >
        <div className={`gp-candidate-visual gp-visual-${candidate.mechanism}`}>
          <span>0{index + 1}</span>
          <Play size={28} />
          <em>
            {candidate.recommended ? "AI 推荐" : `证据分 ${candidate.score}`}
          </em>
        </div>
        <div className="gp-candidate-heading">
          <span>{candidate.name}</span>
          {candidate.recommended ? <i>推荐</i> : null}
        </div>
        <h3>{candidate.hookLine}</h3>
        <p>{candidate.audienceFit}</p>
      </button>
      <div className="gp-beats">
        <b>节奏设计</b>
        {candidate.beats.map((beat) => (
          <div key={beat.id}>
            <time>{beat.range}</time>
            <span>{beat.copy}</span>
          </div>
        ))}
      </div>
      <div className="gp-candidate-evidence">
        <Clock3 />
        证据 {candidate.evidenceRefs.map((id) => evidenceTime(id)).join(" · ")}
      </div>
      <small className="gp-risk">
        <CircleAlert />
        {candidate.risk}
      </small>
      <button className="gp-radio" type="button" onClick={onSelect}>
        {selected ? (
          <>
            <Check />
            已选择
          </>
        ) : (
          "选择此方案"
        )}
      </button>
    </article>
  );
}

function GenerateStep({
  state,
  candidate,
  busy,
  onConfig,
  onGenerate,
  onDownload,
  onBack,
}: {
  state: GamePrerollState;
  candidate?: HookCandidate;
  busy: boolean;
  onConfig: (config: GamePrerollState["config"]) => void;
  onGenerate: () => void;
  onDownload: () => void;
  onBack: () => void;
}) {
  const blockers = generationBlockers(state);
  const succeeded = state.generation.status === "succeeded";
  return (
    <div className="gp-layout gp-generate-layout">
      <main className="gp-panel gp-generation-preview">
        <SectionTitle
          index="05"
          title={succeeded ? "游戏前贴已生成" : "配置并生成前贴"}
          description={
            succeeded
              ? "当前结果仍处于前端模拟阶段，可继续调整参数后重新生成。"
              : "固定事实约束不可修改；创意表现参数可按需要调整。"
          }
        />
        <div className={`gp-phone-preview ${succeeded ? "generated" : ""}`}>
          <div className="gp-phone-content">
            <span>9:16</span>
            <h3>{candidate?.hookLine || "请先选择钩子方案"}</h3>
            <div className="gp-game-stage">
              <i />
              <i />
              <i />
            </div>
            <button type="button" aria-label="播放预览">
              <Play />
            </button>
            <b>{state.config.cta}</b>
          </div>
        </div>
        <p>
          {succeeded
            ? state.generation.diagnostic
            : "预览为创意分镜示意，正式视频将在后端接入 Seedance 后输出。"}
        </p>
        {succeeded ? (
          <div className="gp-result-actions">
            <button className="gp-secondary" type="button" onClick={onGenerate}>
              <RefreshCw />
              调整参数后重生成
            </button>
            <button
              className="gp-primary"
              type="button"
              onClick={onDownload}
            >
              <Download />
              下载前贴视频
            </button>
          </div>
        ) : null}
      </main>
      <aside className="gp-panel gp-generation-config">
        <section>
          <h3>
            <LockKeyhole />
            固定策略约束 <span>不可编辑</span>
          </h3>
          <p>
            <Gamepad2 />
            仅使用原视频中的真实玩法与 UI
          </p>
          <p>
            <ShieldCheck />
            禁止虚构奖励、数值和结果
          </p>
          <p>
            <Download />
            CTA：{state.config.cta}
          </p>
          <p>
            <FileVideo />
            抖音 · 9:16 竖屏
          </p>
        </section>
        <section>
          <h3>
            <Pencil />
            可调生成参数
          </h3>
          <Field label={`前贴时长 · ${state.config.durationSeconds} 秒`}>
            <input
              type="range"
              min="6"
              max="10"
              value={state.config.durationSeconds}
              onChange={(event) =>
                onConfig({
                  ...state.config,
                  durationSeconds: Number(event.target.value) as
                    6 | 7 | 8 | 9 | 10,
                })
              }
            />
            <span className="gp-range-label">
              <i>6秒</i>
              <i>8秒 推荐</i>
              <i>10秒</i>
            </span>
          </Field>
          <Field label="字幕样式">
            <select
              value={state.config.subtitleStyle}
              onChange={(event) =>
                onConfig({
                  ...state.config,
                  subtitleStyle: event.target
                    .value as typeof state.config.subtitleStyle,
                })
              }
            >
              <option value="high_contrast_dynamic">高对比动态字幕</option>
              <option value="minimal_centered">极简居中字幕</option>
            </select>
          </Field>
          <Field label="节奏强度">
            <select
              value={state.config.pace}
              onChange={(event) =>
                onConfig({
                  ...state.config,
                  pace: event.target.value as typeof state.config.pace,
                })
              }
            >
              <option value="balanced">平衡</option>
              <option value="punchy">偏强</option>
              <option value="intense">强烈</option>
            </select>
          </Field>
          <Field label="已选钩子方案">
            <input value={candidate?.name || "未选择"} readOnly />
          </Field>
        </section>
        <div
          className={`gp-generation-ready ${blockers.length ? "blocked" : ""}`}
        >
          {blockers.length ? <CircleAlert /> : <Check />}
          <div>
            <b>
              {blockers.length
                ? "暂不能生成"
                : `${state.evidence.length} 张证据帧已绑定`}
            </b>
            <p>{blockers.join("、") || "固定约束与参数均已就绪"}</p>
          </div>
        </div>
        <button
          className="gp-primary gp-full"
          type="button"
          disabled={busy || blockers.length > 0}
          onClick={onGenerate}
        >
          {busy ? <LoaderCircle className="gp-spin" /> : <WandSparkles />}
          {busy
            ? "正在模拟生成…"
            : `生成 ${state.config.durationSeconds} 秒游戏前贴`}
        </button>
        <button
          className="gp-secondary gp-full"
          type="button"
          disabled={busy}
          onClick={onBack}
        >
          <ArrowLeft />
          返回重新规划候选
        </button>
      </aside>
    </div>
  );
}

function SectionTitle({
  index,
  title,
  description,
}: {
  index?: string;
  title: string;
  description: string;
}) {
  return (
    <div className="gp-section-title">
      {index ? <i>{index}</i> : null}
      <div>
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
    </div>
  );
}
function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <label className="gp-field">
      <span>{label}</span>
      {children}
    </label>
  );
}
function ProvenanceBadge({ value }: { value: Provenance }) {
  return (
    <small className={`gp-provenance ${value}`}>
      {provenanceLabels[value]}
    </small>
  );
}
function formatRange(startMs: number, endMs: number) {
  return `${(startMs / 1000).toFixed(1)}–${(endMs / 1000).toFixed(1)}s`;
}
function evidenceTime(id: string) {
  if (id === "evidence-choice") return "00:20.2";
  if (id === "evidence-fail") return "00:29.8";
  return "00:34.0";
}
