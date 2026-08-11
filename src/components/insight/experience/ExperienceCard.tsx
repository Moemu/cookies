import { useState, type ReactNode } from 'react'
import { ChevronRight } from 'lucide-react'
import type { ApiApplicability, ApiExperience } from '../../../data/api'
import { VerdictBadge } from '../shared'
import { EvidenceTrail } from './EvidenceTrail'

/**
 * 经验卡。
 *
 * **卡面只露三行：结论 / 适用条件 / 凭什么。** 原来的九个字段一个没删，全收进
 * 展开层——列表页上摊开九个字段，一屏放不下三条经验，而查经验的人是来扫一遍
 * 找出哪条跟自己有关的，不是来逐条精读的。
 *
 * 「查」和「管」两个模式共用这一张卡。差别只在 actions 里挂什么按钮，以及查的
 * 时候有 matched 说得出「凭什么推给你」。同一条经验在两屏上长成两个样子的话，
 * 人会以为那是两条不同的经验。
 */
export function ExperienceCard({ experience, matched, status, actions }: {
  experience: ApiExperience
  /**
   * 命中了哪几格适用条件，由后端给。undefined 表示这一屏根本没按条件筛
   * （「管」模式），空数组表示筛了但这条经验哪一格都没设限——两者说出来的话
   * 不一样，所以不能合成一个。
   */
  matched?: string[]
  /** 状态标签。「查」模式不给：那一屏里的经验全是在用的，每张卡上挂一遍是噪音。 */
  status?: ReactNode
  actions?: ReactNode
}) {
  const [open, setOpen] = useState(false)
  const caveat = caveats[experience.verdict]

  return <article className={`experience-card${caveat ? ' experience-card-soft' : ''}`}>
    <header>
      <VerdictBadge judgement={experience}/>
      <h4>{experience.conclusion}</h4>
      {status}
      {experience.needs_review
        // 标记要看得见但不能吓人：这条经验还在用，只是该重新看一眼它的依据了。
        ? <span className="experience-review-flag" title={experience.status_reason}>该看一眼了</span>
        : null}
    </header>

    <p className="experience-scope">
      适用：{formatScope(experience.applicability)}
      {matchNote(experience.applicability, matched)}
    </p>

    {/* 档位不够的要当场说清能拿它干什么，不能只靠徽章上一个 👁 或 ❓。
        没这句话，人会把 👁 读成一条弱一点的结论，照着做，然后归因到它头上。 */}
    {caveat ? <p className="experience-caveat">{caveat}</p> : null}

    <button type="button" className="text-button experience-trail-toggle"
      aria-expanded={open} onClick={() => setOpen(!open)}>
      <ChevronRight size={14} className={open ? 'rotated' : ''}/>
      凭什么
    </button>

    {open ? <EvidenceTrail experience={experience}/> : null}
    {actions ? <footer className="experience-actions">{actions}</footer> : null}
  </article>
}

// 只有 ✅ 能归因的不需要额外提醒——它本来就是「照着做」的那一档。
const caveats: Record<string, string> = {
  observed: '这条只是观察，没排除掉别的变量。可以参考，但别当成「照着做就会这样」。',
  unclear: '这条的样本不够，算不出来是不是真的。当成一个待验证的线索，别当结论用。',
}

/**
 * 适用条件一行文本。
 *
 * 一格里可以有多个取值（一条经验同时适用于抖音和小红书是常态），同一格的多值
 * 用「/」连，格与格之间用「·」隔——都用同一个分隔符的话，「抖音 · 小红书 · 效果广告」
 * 读起来像三个并列的限制，实际是两格。
 */
export function formatScope(applicability: ApiApplicability | undefined): string {
  const parts = applicabilityGroups(applicability).map(group => group.values.join('/'))
  return parts.length ? parts.join(' · ') : '不限'
}

/** 六格适用条件里真正填了值的那些，顺序和后端 matchApplicability 里一致。 */
export function applicabilityGroups(applicability: ApiApplicability | undefined) {
  return ([
    { label: '品牌', key: 'brand', values: applicability?.brands },
    { label: '产品', key: 'product', values: applicability?.products },
    { label: '渠道', key: 'channel', values: applicability?.channels },
    { label: '广告类型', key: 'ad_type', values: applicability?.creative_types },
    { label: '目标', key: 'objective', values: applicability?.objectives },
    { label: '受众', key: 'audience', values: applicability?.audiences },
  ] as const).flatMap(group => group.values?.length
    ? [{ label: group.label, key: group.key, values: group.values }]
    : [])
}

/**
 * 「凭什么推给你」。
 *
 * 三种情况要说成三句话：对上了几格、这条经验本来就没设限、以及这一屏没按条件
 * 筛。后两种都会让 matched 是空的，但意思完全不同——一句「没设限」贴到一条明明
 * 限定了渠道的经验上，人会以为它到处都能用。
 */
function matchNote(applicability: ApiApplicability | undefined, matched: string[] | undefined): ReactNode {
  if (!matched) return null
  if (matched.length) return <small>（{matched.join('、')}对上了）</small>
  if (!applicabilityGroups(applicability).length) return <small>（没设限，任何情况都适用）</small>
  return <small>（没填筛选条件，所以没卡它）</small>
}
