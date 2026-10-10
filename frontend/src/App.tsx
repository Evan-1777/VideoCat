import { useEffect, useState } from 'react'
import { Button, FluentProvider } from '@fluentui/react-components'
import {
  Settings24Regular,
  VideoClip24Regular,
  WeatherMoon24Regular,
  WeatherSunny24Regular,
} from '@fluentui/react-icons'

import ConvertPage from './pages/ConvertPage'
import SettingsPage from './pages/SettingsPage'
import { darkTheme, lightTheme } from './theme'

type Page = 'convert' | 'settings'

export default function App() {
  const [dark, setDark] = useState(() => localStorage.getItem('videocat.theme') === 'dark')
  const [page, setPage] = useState<Page>('convert')

  useEffect(() => {
    document.documentElement.dataset.theme = dark ? 'dark' : 'light'
    localStorage.setItem('videocat.theme', dark ? 'dark' : 'light')
  }, [dark])

  return (
    <FluentProvider theme={dark ? darkTheme : lightTheme} style={{ height: '100%', background: 'transparent' }}>
      <div className="app">
        <header className="titlebar">
          <span className="brandmark" aria-hidden="true">
            <VideoClip24Regular style={{ fontSize: 12 }} />
          </span>
          <span className="brandname">video cat</span>
          <span className="spacer" />
          <Button
            appearance="subtle"
            icon={dark ? <WeatherSunny24Regular /> : <WeatherMoon24Regular />}
            aria-label="切换主题"
            title="切换主题"
            onClick={() => setDark((v) => !v)}
          />
        </header>

        <div className="body">
          <nav className="nav">
            <button className={`nav-item ${page === 'convert' ? 'active' : ''}`} onClick={() => setPage('convert')}>
              <VideoClip24Regular />
              转换
            </button>
            <button className={`nav-item ${page === 'settings' ? 'active' : ''}`} onClick={() => setPage('settings')}>
              <Settings24Regular />
              设置
            </button>
          </nav>

          <main className="main">
            <div className="main-inner">
              {page === 'convert' ? <ConvertPage /> : <SettingsPage />}
            </div>
          </main>
        </div>
      </div>
    </FluentProvider>
  )
}
