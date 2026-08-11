import type { Judgement } from '../../../data/verdict'

// 样本不足占位。空状态不能只写「暂无数据」——那句话既没说缺什么，
// 也没说怎么补。这个占位必须回答这两件事。
export function NotEnoughSample({ judgement, onFindSimilar }: {
  judgement: Judgement
  onFindSimilar?: () => void
}) {
  return (
    <div className="not-enough-sample">
      <p className="not-enough-sample-verdict">❓ {judgement.verdict_label}</p>
      <p className="not-enough-sample-note">{judgement.note}</p>
      {onFindSimilar ? (
        <button type="button" onClick={onFindSimilar}>找相似素材，把样本做厚</button>
      ) : null}
    </div>
  )
}
