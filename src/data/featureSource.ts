// 变量的三种来源，前端侧的统一措辞。后端权威定义在
// internal/systems/insights/assets.go 的 FeatureSource。
//
// 三个词是给不懂实现的人看的：「量出来的」是从素材本身或投放数据算出来的，
// 「人标的」是有人看过并写下的，「模型猜的」是大模型推断的。这个区别不是
// 分类癖——**归因只认前两种**，模型猜的只能参考。所以凡是显示变量取值的
// 地方都要把来源一起显示出来，少一处，那一屏就在悄悄地让人拿推断做决定。

import type { ApiFeatureSource } from './api'

export const featureSourceLabel: Record<ApiFeatureSource, string> = {
  derived: '量出来的',
  human: '人标的',
  ai: '模型猜的',
}

/** 能不能进归因。和后端 FeatureSource.AdmissibleForAttribution() 对齐。 */
export function admissibleForAttribution(source: ApiFeatureSource): boolean {
  return source === 'derived' || source === 'human'
}
