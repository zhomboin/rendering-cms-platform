export const cmsColors = {
  canvas: '#F0F1FA',
  panel: '#F8F8FF',
  well: '#E8EAF5',
  line: '#D6DBED',
  edge: '#7B86A3',
  tableHead: '#EEF0FA',
  ink: '#25345D',
  muted: '#4B5876',
  primary: '#2456C7',
  primaryHover: '#1E49AD',
  primaryPressed: '#183C90',
  focus: '#1D4ED8',
  hover: '#E9EFFD',
  selected: '#E0E9FF',
  disabledBg: '#E8EAF3',
  disabledText: '#737E99',
  success: '#166247',
  successBg: '#E5F4ED',
  warning: '#785006',
  warningBg: '#FFF1D3',
  error: '#9A263E',
  errorBg: '#FBE8EE',
  danger: '#AE2943',
  dangerHover: '#942238',
  dangerPressed: '#7C1B30',
  info: '#204DAB',
  infoBg: '#E8EFFF',
} as const;

// 浮层挂到 body，页面与浮层共用同一组颜色。
export function applyCmsThemeColors(root: HTMLElement) {
  for (const [name, value] of Object.entries(cmsColors)) {
    root.style.setProperty(
      `--cms-${name.replace(/[A-Z]/g, (letter) => `-${letter.toLowerCase()}`)}`,
      value,
    );
  }
}
