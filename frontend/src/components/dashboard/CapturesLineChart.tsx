import { useMemo, useCallback } from 'react';
import { Group } from '@visx/group';
import { LinePath, AreaClosed } from '@visx/shape';
import { curveMonotoneX } from '@visx/curve';
import { scaleTime, scaleLinear } from '@visx/scale';
import { LinearGradient } from '@visx/gradient';
import { AxisBottom, AxisLeft } from '@visx/axis';
import { ParentSize } from '@visx/responsive';
import { TooltipWithBounds, defaultStyles, useTooltip } from '@visx/tooltip';
import { localPoint } from '@visx/event';
import { GridRows } from '@visx/grid';
import { max, extent, bisector } from 'd3-array';
import { timeFormat } from 'd3-time-format';

export interface TimeSeriesPoint {
  date: string;
  value: number;
}

interface ChartBaseProps {
  data: TimeSeriesPoint[];
  width: number;
  height: number;
}

const formatDate = timeFormat("%b %d, %H:%M");
const getDate = (d: TimeSeriesPoint) => new Date(d.date).valueOf();
const getValue = (d: TimeSeriesPoint) => d.value;
const bisectDate = bisector<TimeSeriesPoint, Date>((d) => new Date(d.date)).left;

const tooltipStyles = {
  ...defaultStyles,
  backgroundColor: '#1e293b',
  color: '#f8fafc',
  border: '1px solid #334155',
  borderRadius: '8px',
};

function BaseChart({ data, width, height }: ChartBaseProps) {
  const { tooltipData, tooltipLeft, tooltipTop, tooltipOpen, showTooltip, hideTooltip } = useTooltip<TimeSeriesPoint>();



  const margin = { top: 20, right: 20, bottom: 40, left: 40 };
  const innerWidth = Math.max(0, width - margin.left - margin.right);
  const innerHeight = Math.max(0, height - margin.top - margin.bottom);

  const dateScale = useMemo(() => scaleTime({
    range: [0, innerWidth],
    domain: extent(data, getDate) as [number, number],
  }), [innerWidth, data]);

  const valueMax = max(data, getValue) || 0;
  const valueScale = useMemo(() => scaleLinear({
    range: [innerHeight, 0],
    domain: [0, valueMax + Math.ceil(valueMax * 0.2) + 1],
    nice: true,
  }), [innerHeight, valueMax]);

  const handleTooltip = useCallback((event: React.TouchEvent<SVGRectElement> | React.MouseEvent<SVGRectElement>) => {
    const { x } = localPoint(event) || { x: 0 };
    const x0 = dateScale.invert(x - margin.left);
    const index = bisectDate(data, x0, 1);
    const d0 = data[index - 1];
    const d1 = data[index];
    let d = d0;
    if (d1 && d0) {
      d = x0.valueOf() - getDate(d0) > getDate(d1) - x0.valueOf() ? d1 : d0;
    }
    if(d){
      showTooltip({
        tooltipData: d,
        tooltipLeft: x,
        tooltipTop: valueScale(getValue(d)) + margin.top,
      });
    }
  }, [showTooltip, valueScale, dateScale, data, margin]);

  return (
    <div style={{ position: 'relative' }}>
      <svg width={width} height={height}>
        <LinearGradient id="area-gradient" from="#3b82f6" to="#3b82f6" fromOpacity={0.3} toOpacity={0} />
        <Group left={margin.left} top={margin.top}>
          <GridRows scale={valueScale} width={innerWidth} height={innerHeight} stroke="#1e293b" />
          <AxisLeft scale={valueScale} stroke="#334155" tickStroke="#334155" tickLabelProps={() => ({ fill: '#94a3b8', fontSize: 10, textAnchor: 'end', dy: '0.33em' })} />
          <AxisBottom top={innerHeight} scale={dateScale} stroke="#334155" tickStroke="#334155" tickLabelProps={() => ({ fill: '#94a3b8', fontSize: 10, textAnchor: 'middle' })} />

          <AreaClosed<TimeSeriesPoint>
            data={data}
            x={(d) => dateScale(getDate(d)) ?? 0}
            y={(d) => valueScale(getValue(d)) ?? 0}
            yScale={valueScale}
            strokeWidth={0}
            fill="url(#area-gradient)"
            curve={curveMonotoneX}
          />
          <LinePath<TimeSeriesPoint>
            data={data}
            x={(d) => dateScale(getDate(d)) ?? 0}
            y={(d) => valueScale(getValue(d)) ?? 0}
            stroke="#3b82f6"
            strokeWidth={2}
            curve={curveMonotoneX}
          />

          <rect
            x={0}
            y={0}
            width={innerWidth}
            height={innerHeight}
            fill="transparent"
            onTouchStart={handleTooltip}
            onTouchMove={handleTooltip}
            onMouseMove={handleTooltip}
            onMouseLeave={() => hideTooltip()}
          />

          {tooltipData && (
            <circle
              cx={dateScale(getDate(tooltipData))}
              cy={valueScale(getValue(tooltipData))}
              r={4}
              fill="#ec4899"
              stroke="white"
              strokeWidth={2}
              pointerEvents="none"
            />
          )}
        </Group>
      </svg>
      {tooltipOpen && tooltipData && (
        <TooltipWithBounds top={tooltipTop} left={tooltipLeft} style={tooltipStyles}>
          <div className="text-sm font-bold">{getValue(tooltipData)} captures</div>
          <div className="text-xs text-slate-400">{formatDate(new Date(tooltipData.date))}</div>
        </TooltipWithBounds>
      )}
    </div>
  );
}

export function CapturesLineChart({ data }: { data: TimeSeriesPoint[] }) {
  if (!data || data.length === 0) {
    return <div className="w-full h-full flex items-center justify-center text-slate-500">No data available</div>;
  }
  return (
    <div className="w-full h-full min-h-[300px]">
      <ParentSize>
        {({ width, height }) => <BaseChart data={data} width={width} height={height} />}
      </ParentSize>
    </div>
  );
}
