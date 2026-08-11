import { useState } from 'react'
import { Search } from 'lucide-react'
import type { ApiSimilarAssetResult } from '../../../data/api'
import { SimilarPanel } from '../assets/SimilarPanel'

/**
 * ❓「算不出来」的升级通道，就地展开。
 *
 * ❓ 缺的从来是样本，不是算法。这个动作的意思是「从库里把同样取值的素材拉过来，
 * 让这条结论有足够的样本重算一遍」。
 *
 * **就地展开，不跳页。**跳到「素材 · 找相似」那一屏也能查同样的东西，但人一跳走
 * 就丢了刚才在看的那条结论——回来之后还得自己想起来当时问的是什么。
 *
 * 第二次按收起：结果面板不短，几条 ❓ 全展开之后这一屏就没法读了。
 */
export function FindSimilarAction({ probe }: {
  /** 按什么去找由调用方决定：驱动因素按那一个变量，素材对比按全部可归因的变量。 */
  probe: () => Promise<ApiSimilarAssetResult>
}) {
  const [result, setResult] = useState<ApiSimilarAssetResult | null>(null)
  const [state, setState] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle')
  const [notice, setNotice] = useState('')

  const toggle = () => {
    if (state === 'loading') return
    if (result) {
      setResult(null)
      setState('idle')
      setNotice('')
      return
    }
    setState('loading')
    setNotice('')
    probe()
      .then(value => { setResult(value); setState('ready') })
      .catch(cause => {
        setResult(null)
        setState('error')
        // 失败要说出来。静悄悄什么都不展开的话，人会以为「库里真的没有像它的」。
        setNotice(cause instanceof Error ? cause.message : '没找成，稍后再试。')
      })
  }

  return <div className="find-similar">
    <button type="button" className="text-button" onClick={toggle} disabled={state === 'loading'}>
      <Search size={13}/>
      {state === 'loading' ? '正在找…' : result ? '收起相似素材' : '找相似素材，把样本做厚'}
    </button>
    {notice ? <p className="form-error">{notice}</p> : null}
    {result ? <SimilarPanel result={result}/> : null}
  </div>
}
