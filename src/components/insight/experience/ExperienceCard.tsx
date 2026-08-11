import { useState, type ReactNode } from 'react'
import { ChevronRight } from 'lucide-react'
import type { ApiApplicability, ApiExperience, ApiExperienceMatch } from '../../../data/api'
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
 * 时候有 match.matched 说得出「凭什么推给你」。同一条经验在两屏上长成两个样子
 * 的话，人会以为那是两条不同的经验。
 */
export function ExperienceCard({ match, actions }: {
  match: ApiExperienceMatch
  actions?: ReactNode
}) {
  const [open, setOpen] = useState(false)
  const { experience } = match

  return <article className={`experience-card${match.default ? '' : ' experience-card-observed'}`}>
    <header>
      <VerdictBadge judgement={experience}/>
      <h4>{experience.conclusion}</h4>
      {experience.needs_review
        // 标记要看得见但不能吓人：这条经验还在用，只是该重新看一眼它的依据了。
        ? <span className="experience-review-flag" title={experience.status_reason}>该看一眼了</span>
        : null}
    </header>

    <p className="experience-scope">
      适用：{formatScope(experience.applicability)}
      {matchNote(match)}
    </p>

    {/* 只是观察的要当场说清能拿它干什么，不能只靠徽章上一个 👁。
        没这句话，人会把它读成一条弱一点的结论，照着做，然后归因到它头上。 */}
    {!match.default ? <p className="experience-caveat">
      这条只是观察，没排除掉别的变量。可以参考，但别当成「照着做就会这样」。
    </p> : null}

    <button type="button" className="text-button experience-trail-toggle"
      aria-expanded={open} onClick={() => setOpen(!open)}>
      <ChevronRight size={14} className={open ? 'rotated' : ''}/>
      凭什么
    </button>

    {open ? <EvidenceTrail experience={experience}/> : null}
    {actions ? <footer className="experience-actions">{actions}</footer> : null}
  </article>
}

/**
 * 适用条件一行文本。
 *
 * 一格里可以有多个取值（一条经验同时适用于抖音和小红书是常态），同一格的多值
 * 用「/」连，格与格之间用「·」隔——都用同一个分隔符的话，「抖音 · 小红书 · 效果广告」
 * 读起来像三个并列的限制，实际是两格。
 */
export function formatScope(applicability: ApiApplicability | undefined): string {
  const groups = [
    applicability?.brands,
    applicability?.products,
    applicability?.channels,
    applicability?.creative_types,
    applicability?.objectives,
    applicability?.audiences,
  ]
  const parts = groups
    .filter((values): values is string[] => Boolean(values?.length))
    .map(values => values.join('/'))
  return parts.length ? parts.join(' · ') : '不限'
}

/**
 * 「凭什么推给你」。
 *
 * 三种情况要说成三句话：对上了几格、这条经验本来就没设限、以及查的人什么条件
 * 都没填。后两种都会让 matched 是空的，但意思完全不同——一句「没设限」贴到一条
 * 明明限定了渠道的经验上，人会以为它到处都能用。
 */
function matchNote(match: ApiExperienceMatch): ReactNode {
  if (match.matched.length) return <small>（{match.matched.join('、')}对上了）</small>
  if (isUnrestricted(match.experience.applicability)) return <small>（没设限，任何情况都适用）</small>
  return <small>（没按条件筛，这是这个 Project 里的全部）</small>
}

function isUnrestricted(applicability: ApiApplicability | undefined): boolean {
  return formatScope(applicability) === '不限'
}
