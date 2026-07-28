import { describe, expect, it } from 'vitest'
import {
  activeBusinessModule,
  creativePreRollPath,
  creativeTaskPath,
  deliveryPlanPath,
  destinationForProject,
  insightsPath,
  projectHomePath,
  routeProjectId,
} from './routes'

describe('application route manifest', () => {
  it('keeps the project identity in stable business URLs', () => {
    expect(projectHomePath('project spring')).toBe('/projects/project%20spring/home')
    expect(creativeTaskPath('project_1', 'task_1', 'production')).toBe('/projects/project_1/creative/tasks/task_1/production')
    expect(creativePreRollPath('project spring')).toBe('/projects/project%20spring/creative/video/performance/pre-roll')
    expect(deliveryPlanPath('project_1', 'plan_1')).toBe('/projects/project_1/delivery/plans/plan_1')
    expect(insightsPath('project_1', 'reports')).toBe('/projects/project_1/insights/reports')
    expect(routeProjectId('/projects/project_1/creative/tasks/task_1/review')).toBe('project_1')
  })

  it('preserves the current business destination when switching projects', () => {
    expect(destinationForProject('/projects/old/creative/tasks/task_1/content', 'new')).toBe('/projects/new/creative/tasks')
    expect(destinationForProject('/projects/old/manage', 'new')).toBe('/projects/new/manage')
    expect(destinationForProject('/projects/old/delivery/plans/plan_1', 'new')).toBe('/projects/new/delivery/plans')
    expect(destinationForProject('/projects/old/insights/reports', 'new')).toBe('/projects/new/insights/prelaunch')
    expect(destinationForProject('/projects/old/home', 'new')).toBe('/projects/new/home')
  })

  it('does not promote support pages into a global business module', () => {
    expect(activeBusinessModule('/projects/p1/assets')).toBe('creative')
    expect(activeBusinessModule('/projects/p1/provider-jobs')).toBe('creative')
    expect(activeBusinessModule('/projects/p1/home')).toBe('projects')
    expect(activeBusinessModule('/projects/p1/delivery/plans')).toBe('delivery')
    expect(activeBusinessModule('/projects/p1/insights/performance')).toBe('insights')
  })
})
