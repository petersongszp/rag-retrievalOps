import dayjs, { type Dayjs } from 'dayjs';
import type { CostTimeseriesPoint } from '@/types/kb';

export type CostViewMode = 'day' | 'month';
export type CostDisplayMode = 'chart' | 'list';

export type CostWindowParams = {
  start_time: string;
  end_time: string;
  bucket: '1h' | '1d';
  tz: 'Asia/Shanghai';
};

export type CostChartDatum = {
  bucket: string;
  label: string;
  value: number;
  tokensPer1KQueries?: number;
  avgTokensPerQuery?: number;
};

export type CostChartBar = {
  x: number;
  y: number;
  width: number;
  height: number;
  label: string;
  value: number;
};

export type CostChartPoint = {
  x: number;
  y: number;
  label: string;
  value: number;
};

export type CostChartTick = {
  x: number;
  label: string;
  show: boolean;
};

export type CostChartHitArea = {
  x: number;
  y: number;
  width: number;
  height: number;
  label: string;
  value: number;
};

export type CostChartGeometry = {
  width: number;
  height: number;
  plotLeft: number;
  plotRight: number;
  plotTop: number;
  plotBottom: number;
  baselineY: number;
  maxValue: number;
  yTicks: number[];
  xTicks: CostChartTick[];
  bars: CostChartBar[];
  points: CostChartPoint[];
  hitAreas: CostChartHitArea[];
  linePath: string;
};

export const SHANGHAI_TZ = 'Asia/Shanghai';
const SHANGHAI_OFFSET = '+08:00';

function getShanghaiCalendarParts(date = new Date()) {
  const formatter = new Intl.DateTimeFormat('en-CA', {
    timeZone: SHANGHAI_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  });
  const parts = formatter.formatToParts(date);

  const partValue = (type: 'year' | 'month' | 'day') =>
    parts.find((part) => part.type === type)?.value ?? '';

  return {
    year: partValue('year'),
    month: partValue('month'),
    day: partValue('day'),
  };
}

export function getDefaultCostSelectedDate(now = new Date()): Dayjs {
  const { year, month, day } = getShanghaiCalendarParts(now);
  return dayjs(`${year}-${month}-${day}`);
}

export function buildCostWindowParams(
  viewMode: CostViewMode,
  selectedDate: Dayjs
): CostWindowParams {
  const anchor = selectedDate.isValid() ? selectedDate : getDefaultCostSelectedDate();
  if (viewMode === 'month') {
    const monthStart = anchor.startOf('month');
    const monthEnd = anchor.endOf('month');
    return {
      start_time: `${monthStart.format('YYYY-MM-DD')}T00:00:00${SHANGHAI_OFFSET}`,
      end_time: `${monthEnd.format('YYYY-MM-DD')}T23:59:59.999999999${SHANGHAI_OFFSET}`,
      bucket: '1d',
      tz: SHANGHAI_TZ,
    };
  }

  return {
    start_time: `${anchor.format('YYYY-MM-DD')}T00:00:00${SHANGHAI_OFFSET}`,
    end_time: `${anchor.format('YYYY-MM-DD')}T23:59:59.999999999${SHANGHAI_OFFSET}`,
    bucket: '1h',
    tz: SHANGHAI_TZ,
  };
}

export function formatCostAxisLabel(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return '0';
  }
  if (value >= 1000000) {
    return `${(value / 1000000).toFixed(1)}M`;
  }
  if (value >= 1000) {
    return `${(value / 1000).toFixed(1)}K`;
  }
  if (value >= 1) {
    return value.toFixed(2);
  }
  if (value >= 0.01) {
    return value.toFixed(3);
  }
  return value.toFixed(4);
}

export function formatCostBucketLabel(bucket: string, viewMode: CostViewMode): string {
  const parsed = dayjs(bucket);
  if (!parsed.isValid()) {
    return bucket;
  }
  return viewMode === 'month' ? parsed.format('MM-DD') : parsed.format('HH:mm');
}

export function buildCostListRows(items: CostTimeseriesPoint[]): CostTimeseriesPoint[] {
  return [...items]
    .filter((item) => (item.total_estimated_cost ?? 0) > 0)
    .sort((left, right) => dayjs(right.bucket).valueOf() - dayjs(left.bucket).valueOf());
}

function buildSmoothLinePath(points: CostChartPoint[]): string {
  if (points.length === 0) {
    return '';
  }
  if (points.length === 1) {
    return `M ${points[0].x} ${points[0].y}`;
  }

  let path = `M ${points[0].x} ${points[0].y}`;
  for (let index = 0; index < points.length - 1; index += 1) {
    const current = points[index];
    const next = points[index + 1];
    const controlX = (current.x + next.x) / 2;
    path += ` C ${controlX} ${current.y}, ${controlX} ${next.y}, ${next.x} ${next.y}`;
  }
  return path;
}

export function buildCostChartGeometry(data: CostChartDatum[]): CostChartGeometry {
  const width = 960;
  const height = 360;
  const plotLeft = 64;
  const plotRight = 32;
  const plotTop = 24;
  const plotBottom = 44;
  const plotWidth = width - plotLeft - plotRight;
  const plotHeight = height - plotTop - plotBottom;
  const baselineY = plotTop + plotHeight;
  const values = data.map((item) => item.value);
  const maxValue = Math.max(...values, 0);
  const safeMaxValue = maxValue > 0 ? maxValue : 1;
  const slotWidth = data.length > 0 ? plotWidth / data.length : plotWidth;
  const barWidth = Math.max(Math.min(slotWidth * 0.42, 24), 8);
  const visibleTickCount = 6;
  const tickStep = data.length <= visibleTickCount ? 1 : Math.ceil(data.length / visibleTickCount);

  const points = data.map((item, index) => {
    const centerX = plotLeft + slotWidth * index + slotWidth / 2;
    const y = baselineY - (item.value / safeMaxValue) * plotHeight;
    return {
      x: centerX,
      y,
      label: item.label,
      value: item.value,
    };
  });

  const bars = data.map((item, index) => {
    const point = points[index];
    const barHeight = item.value > 0 ? (item.value / safeMaxValue) * plotHeight : 0;
    return {
      x: point.x - barWidth / 2,
      y: baselineY - barHeight,
      width: barWidth,
      height: barHeight,
      label: item.label,
      value: item.value,
    };
  });

  const xTicks = data.map((item, index) => ({
    x: points[index]?.x ?? plotLeft,
    label: item.label,
    show: index === 0 || index === data.length - 1 || index % tickStep === 0,
  }));

  const hitAreas = data.map((item, index) => ({
    x: plotLeft + slotWidth * index,
    y: plotTop,
    width: slotWidth,
    height: plotHeight,
    label: item.label,
    value: item.value,
  }));

  const yTicks = Array.from({ length: 5 }, (_, index) => (safeMaxValue / 4) * index).reverse();

  return {
    width,
    height,
    plotLeft,
    plotRight,
    plotTop,
    plotBottom,
    baselineY,
    maxValue: safeMaxValue,
    yTicks,
    xTicks,
    bars,
    points,
    hitAreas,
    linePath: buildSmoothLinePath(points),
  };
}
