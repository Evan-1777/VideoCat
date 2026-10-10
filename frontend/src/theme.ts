import { BrandVariants, createDarkTheme, createLightTheme, Theme } from '@fluentui/react-components'

// Win11 强调色 #0078D4 对应的品牌色阶。
const videocatBrand: BrandVariants = {
  10: '#001322',
  20: '#00243d',
  30: '#003156',
  40: '#003d6e',
  50: '#004a87',
  60: '#0057a0',
  70: '#0065ba',
  80: '#0078d4',
  90: '#1a86db',
  100: '#2f94e2',
  110: '#44a2e9',
  120: '#59b0f0',
  130: '#6ebef7',
  140: '#83ccfe',
  150: '#98daff',
  160: '#ade8ff',
}

export const lightTheme: Theme = {
  ...createLightTheme(videocatBrand),
  fontFamilyBase: '"Segoe UI", system-ui, -apple-system, sans-serif',
}

export const darkTheme: Theme = {
  ...createDarkTheme(videocatBrand),
  fontFamilyBase: '"Segoe UI", system-ui, -apple-system, sans-serif',
}

darkTheme.colorBrandForeground1 = videocatBrand[110]
darkTheme.colorBrandForeground2 = videocatBrand[120]
