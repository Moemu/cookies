import { Component, type ErrorInfo, type ReactNode } from 'react'
import { CircleAlert, RefreshCw } from 'lucide-react'

/**
 * 渲染错误边界。
 *
 * React 里任何一个组件在渲染期间抛错，如果没有边界接住，整棵树会被卸载——
 * 页面直接白屏，连导航都点不了。一个字段的空值处理疏漏不该让整个产品消失。
 *
 * 这一层只负责「别让局部错误炸掉全局」，不负责修数据问题：错误信息照实显示出来，
 * 好让人能把它报回来，而不是看到一句无信息量的「出错了」。
 *
 * 与 StateBoundary 的分工：StateBoundary 处理已知的数据状态（加载/空/错误/无权限），
 * 是业务预期内的分支；这里处理的是预期之外的代码异常。
 */
interface RenderErrorBoundaryProps {
  children: ReactNode
  /** 出错区域的名字，显示给用户，便于定位是哪一块坏了。 */
  contextLabel?: string
  /** 这个值一变就清空错误状态——切换页面时不该还卡在上一页的错误上。 */
  resetKey?: string
}

interface RenderErrorBoundaryState {
  error: Error | null
  resetKey?: string
}

export class RenderErrorBoundary extends Component<RenderErrorBoundaryProps, RenderErrorBoundaryState> {
  state: RenderErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): Partial<RenderErrorBoundaryState> {
    return { error }
  }

  static getDerivedStateFromProps(
    props: RenderErrorBoundaryProps,
    state: RenderErrorBoundaryState,
  ): Partial<RenderErrorBoundaryState> | null {
    // 换了页面就重新给一次机会，否则用户会被永久钉在错误屏上。
    if (props.resetKey !== state.resetKey) return { error: null, resetKey: props.resetKey }
    return null
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // 保留组件栈：线上是压缩过的代码，没有这段栈基本无从下手。
    console.error('[RenderErrorBoundary] 渲染失败：', error, info.componentStack)
  }

  render() {
    const { error } = this.state
    if (!error) return this.props.children

    const contextLabel = this.props.contextLabel ?? '这个区域'
    return <div className="state-surface state-error" role="alert" aria-label={`${contextLabel}渲染失败`}>
      <CircleAlert size={28} aria-hidden="true"/>
      <span className="state-context">{contextLabel}</span>
      <h2>这一块没能显示出来</h2>
      <p>
        页面渲染时出错了，其它区域不受影响，你可以继续用左边的导航。
        这通常是某个字段的数据形状和前端预期不一致，不是你的操作有问题。
      </p>
      <p className="state-error-detail"><code>{error.message}</code></p>
      <button className="secondary-button" onClick={() => this.setState({ error: null })}>
        <RefreshCw size={15} aria-hidden="true"/>重新渲染
      </button>
    </div>
  }
}
