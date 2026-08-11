import type { ApiSimilarAssetResult } from '../../../data/api'
import type { Judgement } from '../../../data/verdict'
import { FindSimilarAction } from './FindSimilarAction'

// 样本不足占位。空状态不能只写「暂无数据」——那句话既没说缺什么，
// 也没说怎么补。这个占位必须回答这两件事。
export function NotEnoughSample({ judgement, onFindSimilar }: {
  judgement: Judgement
  // 给了才出按钮，点了在原地展开相似素材。整屏一条都没有的视图不传：那时候
  // 连一个变量取值都没有，按什么去找都答不上来。
  onFindSimilar?: () => Promise<ApiSimilarAssetResult>
}) {
  return (
    <div className="not-enough-sample">
      <p className="not-enough-sample-verdict">❓ {judgement.verdict_label}</p>
      <p className="not-enough-sample-note">{judgement.note}</p>
      {onFindSimilar ? <FindSimilarAction probe={onFindSimilar}/> : null}
    </div>
  )
}
