declare module 'apexcharts' {
  export type ApexNonAxisChartSeries = number[];
  export type ApexAxisChartSeries = { name?: string; type?: string; color?: string; data: number[] }[];

  export interface ApexOptions {
    series?: ApexNonAxisChartSeries | ApexAxisChartSeries;
    labels?: string[];
    colors?: string[];
    chart?: {
      type?: 'line' | 'area' | 'bar' | 'pie' | 'donut' | 'radialBar' | 'scatter' | 'bubble' | 'kline' | 'candlestick' | 'boxPlot' | 'rangeBar' | 'rangeArea' | 'treemap' | 'heatmap' | 'polygon' | 'radar';
      height?: number | string;
      width?: number | string;
      foreColor?: string;
      background?: string;
      group?: string;
      id?: string;
      toolbar?: { show?: boolean; tools?: { download?: boolean; selection?: boolean; zoom?: boolean; pan?: boolean; reset?: boolean; zoomIn?: boolean; zoomOut?: boolean; } };
      animations?: { enabled?: boolean; easing?: 'linear' | 'easein' | 'easeout' | 'easeinout' | 'swing' | 'bounce'; speed?: number; animateGradually?: { enabled?: boolean; delay?: number }; dynamicAnimation?: { enabled?: boolean; speed?: number } };
    };
    plotOptions?: {
      bar?: { horizontal?: boolean; columnWidth?: string | number; barHeight?: string | number; distributed?: boolean; borderRadius?: number | { topLeft?: number; topRight?: number; bottomLeft?: number; bottomRight?: number } };
      pie?: { donut?: { labels?: { show?: boolean; name?: { show?: boolean; fontSize?: string; fontFamily?: string; fontWeight?: number | string; color?: string; offsetY?: number }; value?: { show?: boolean; fontSize?: string; fontFamily?: string; fontWeight?: number | string; color?: string; offsetY?: number; formatter?: (val: number) => string }; total?: { show?: boolean; showAlways?: boolean; label?: string; fontSize?: string; fontFamily?: string; fontWeight?: number | string; color?: string; formatter?: (w: any) => string } } } };
    };
    dataLabels?: { enabled?: boolean };
    stroke?: { curve?: 'smooth' | 'straight' | 'monotoneCubic' | 'stepline' | 'linear'; width?: number | number[]; show?: boolean };
    xaxis?: { categories?: string[] | number[]; labels?: { style?: { colors?: string | string[]; fontSize?: string } } };
    yaxis?: { labels?: { style?: { colors?: string | string[] }; formatter?: (val: number) => string } };
    legend?: { position?: 'top' | 'right' | 'bottom' | 'left'; labels?: { colors?: string } };
    grid?: { borderColor?: string };
    theme?: { mode?: 'light' | 'dark' };
    responsive?: { breakpoint?: number; options?: ApexOptions }[];
    tooltip?: {
      theme?: 'light' | 'dark' | 'colored';
      enabled?: boolean;
      shared?: boolean;
      intersect?: boolean;
      followCursor?: boolean;
      y?: { formatter?: (val: number) => string };
    };
  }

  /**
   * ApexCharts 类，用于创建和管理图表实例
   */
  export class ApexCharts {
    constructor(el: HTMLElement | string, options: ApexOptions);
    render(): Promise<void>;
    destroy(): void;
    updateSeries(series: number[] | { name?: string; data?: number[] }[], animate?: boolean): void;
    updateOptions(options: ApexOptions, animate?: boolean, updateSyncedCharts?: boolean): void;
    appendData(data: number[] | { name?: string; data?: number[] }[]): void;
    toggleSeries(seriesName: string): void;
    showSeries(seriesName: string): void;
    hideSeries(seriesName: string): void;
    zoomX(start: number, end: number): void;
    resetSeries(): void;
  }

  export default ApexCharts;
}
