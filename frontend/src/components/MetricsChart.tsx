import { useEffect, useRef, useState, type ReactNode } from 'react'
import * as echarts from 'echarts/core'
import { LineChart, BarChart, HeatmapChart } from 'echarts/charts'
import { AriaComponent, GridComponent, LegendComponent, TooltipComponent, VisualMapComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { EChartsOption } from 'echarts'
import { cn } from '@/lib/utils'

echarts.use([LineChart, BarChart, HeatmapChart, AriaComponent, GridComponent, LegendComponent, TooltipComponent, VisualMapComponent, CanvasRenderer])

interface MetricsChartProps {
  option: EChartsOption
  ariaLabel: string
  className?: string
  fallback?: ReactNode
}

// MetricsChart is the single ECharts lifecycle adapter. It is imported only by
// the lazy Metrics route, disposes Canvas instances on unmount, and lets the
// parent provide an equivalent accessible fallback when rendering fails.
export function MetricsChart({ option, ariaLabel, className, fallback }: MetricsChartProps) {
  const elementRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<echarts.ECharts>()
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    const element = elementRef.current
    if (!element || failed) return
    try {
      const chart = echarts.init(element, undefined, { renderer: 'canvas' })
      chartRef.current = chart
      chart.setOption(option, { notMerge: true, lazyUpdate: true })
      const observer = new ResizeObserver(() => chart.resize())
      observer.observe(element)
      return () => {
        observer.disconnect()
        chart.dispose()
        chartRef.current = undefined
      }
    } catch {
      setFailed(true)
      return undefined
    }
  }, [failed])

  useEffect(() => {
    if (!chartRef.current || failed) return
    try {
      chartRef.current.setOption(option, { notMerge: true, lazyUpdate: true })
    } catch {
      setFailed(true)
    }
  }, [option, failed])

  if (failed) return <div className={cn('flex min-h-32 items-center', className)}>{fallback ?? <p className="text-xs text-zinc-500">Chart unavailable.</p>}</div>
  return <div ref={elementRef} role="img" aria-label={ariaLabel} className={cn('min-h-32 w-full', className)} />
}
