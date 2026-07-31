import { describe, expect, it } from 'vitest'
import {
  activeBusinessModule,
  creativePreRollPath,
  creativeTaskPath,
  deliveryPlanPath,
  destinationForProject,
  insightEntries,
  insightEntry,
  insightExperiencePath,
  insightGroups,
  insightPath,
  insightViewPath,
  projectHomePath,
  routeProjectId,
} from './routes'

describe('application route manifest', () => {
  it('keeps the project identity in stable business URLs', () => {
    expect(projectHomePath('project spring')).toBe('/projects/project%20spring/home')
    expect(creativeTaskPath('project_1', 'task_1', 'production')).toBe('/projects/project_1/creative/tasks/task_1/production')
    expect(creativePreRollPath('project spring')).toBe('/projects/project%20spring/creative/video/performance/pre-roll')
    expect(deliveryPlanPath('project_1', 'plan_1')).toBe('/projects/project_1/delivery/plans/plan_1')
    expect(insightPath('project_1', 'reports')).toBe('/projects/project_1/insight/reports')
    expect(routeProjectId('/projects/project_1/creative/tasks/task_1/review')).toBe('project_1')
  })

  it('preserves the current business destination when switching projects', () => {
    expect(destinationForProject('/projects/old/creative/tasks/task_1/content', 'new')).toBe('/projects/new/creative/tasks')
    expect(destinationForProject('/projects/old/manage', 'new')).toBe('/projects/new/manage')
    expect(destinationForProject('/projects/old/delivery/plans/plan_1', 'new')).toBe('/projects/new/delivery/plans')
    expect(destinationForProject('/projects/old/insight/reports', 'new')).toBe('/projects/new/insight/prelaunch')
    expect(destinationForProject('/projects/old/home', 'new')).toBe('/projects/new/home')
  })

  it('does not promote support pages into a global business module', () => {
    expect(activeBusinessModule('/projects/p1/assets')).toBe('creative')
    expect(activeBusinessModule('/projects/p1/provider-jobs')).toBe('creative')
    expect(activeBusinessModule('/projects/p1/home')).toBe('projects')
    expect(activeBusinessModule('/projects/p1/delivery/plans')).toBe('delivery')
    expect(activeBusinessModule('/projects/p1/insight/performance')).toBe('insights')
    // 洞察下的分析素材库不能被当成创意模块的项目素材页。
    expect(activeBusinessModule('/projects/p1/insight/assets')).toBe('insights')
    // 二级视图的名字里带 strategy / creative，不能因此跳到别的模块。
    expect(activeBusinessModule('/projects/p1/insight/prelaunch/strategy-evidence')).toBe('insights')
    expect(activeBusinessModule('/projects/p1/insight/prelaunch/creative-suggestions')).toBe('insights')
    expect(activeBusinessModule('/strategy')).toBe('strategy')
    expect(activeBusinessModule('/strategy/projects/p1')).toBe('strategy')
    expect(activeBusinessModule('/projects')).toBe('projects')
    expect(destinationForProject('/projects/old/insight/prelaunch/strategy-evidence', 'new'))
      .toBe('/projects/new/insight/prelaunch')
  })

  it('exposes the eleven insight entries in the five documented groups', () => {
    expect(insightGroups.map((group) => group.label)).toEqual(['工作', '数据', '素材与分析', '经验与输出', '治理'])
    expect(insightEntries.map((entry) => entry.label)).toEqual([
      '投前洞察', '投后分析', '数据接入', '分析素材库', '内容分析',
      '实验中心', '经验库', '报告中心', '数据质量', '能力运营', '系统设置',
    ])
    expect(insightEntries.filter((entry) => entry.built).map((entry) => entry.key))
      .toEqual(['prelaunch', 'performance', 'experiences', 'reports'])
    expect(insightPath('project_1', 'data-quality')).toBe('/projects/project_1/insight/data-quality')
    expect(insightExperiencePath('project_1', 'needs-review')).toBe('/projects/project_1/insight/experiences/needs-review')
  })

  it('keeps every documented second-level view reachable on its own URL', () => {
    expect(insightEntry('performance')?.views.map((view) => view.label))
      .toEqual(['指标总览', '素材对比', 'Cohort', '趋势', '疲劳', '异常'])
    expect(insightEntry('reports')?.views.filter((view) => view.built).map((view) => view.label))
      .toEqual(['任务复盘'])
    // 一级入口是否可用，由它下面有没有做完的二级视图决定，不再单独维护。
    expect(insightEntry('data-quality')?.views.some((view) => view.built)).toBe(false)
    expect(insightViewPath('project_1', 'prelaunch')).toBe('/projects/project_1/insight/prelaunch/strategy-evidence')
    expect(insightViewPath('project_1', 'performance', 'trends')).toBe('/projects/project_1/insight/performance/trends')
  })
})
