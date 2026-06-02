import { useState, useEffect, useRef, useCallback } from 'react'
import { GetVideos, GetMagnets } from '../wailsjs/go/main/App'

interface Video {
  title: string
  code: string
  cover: string
  detailUrl: string
}

interface Magnet {
  name?: string
  link?: string
  size?: string
  date?: string
  tags?: string[]
  source?: string
}

interface MagnetResult {
  code: string
  query?: string
  magnets?: Magnet[]
  detailUrl?: string
  blocked?: boolean
  note?: string
}

type MagnetStatus = 'waiting' | 'loading' | 'done' | 'error'

/* ── Splash screen ─────────────────────────────────────── */

function Splash({ onFinish }: { onFinish: () => void }) {
  const [phase, setPhase] = useState<'logo' | 'fadeout'>('logo')

  useEffect(() => {
    const t1 = setTimeout(() => setPhase('fadeout'), 1400)
    const t2 = setTimeout(onFinish, 2000)
    return () => { clearTimeout(t1); clearTimeout(t2) }
  }, [onFinish])

  return (
    <div className={`splash ${phase === 'fadeout' ? 'splash-out' : ''}`}>
      <div className="splash-content">
        <div className="splash-icon">
          <svg width="56" height="56" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <polygon points="5 3 19 12 5 21 5 3" />
          </svg>
        </div>
        <h1 className="splash-title">CosAvLink</h1>
        <p className="splash-sub">cosplay.jav.pw · javdb.com</p>
        <div className="splash-dots">
          <span /><span /><span />
        </div>
      </div>
    </div>
  )
}

/* ── Main App ──────────────────────────────────────────── */

function App() {
  const [showSplash, setShowSplash] = useState(true)
  const [page, setPage] = useState(1)
  const [videos, setVideos] = useState<Video[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<number | null>(null)
  const [pageDir, setPageDir] = useState<'left' | 'right' | null>(null)

  const [magnetStates, setMagnetStates] = useState<Record<string, { status: MagnetStatus; data?: MagnetResult }>>({})
  const prefetchedRef = useRef<Map<number, Video[]>>(new Map())

  /* ── Data loading ─── */

  const loadPage = useCallback(async (pageNum: number) => {
    setLoading(true)
    setError('')
    setSelected(null)
    try {
      const data = await GetVideos(pageNum) as unknown as Video[]
      if (!data || data.length === 0) {
        setError('没有更多视频了')
        setVideos([])
      } else {
        setVideos(data)
      }
    } catch (e: any) {
      setError('加载失败：' + String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  const goToPage = useCallback((pageNum: number) => {
    if (pageNum < 1) return
    setPageDir(pageNum > page ? 'right' : 'left')
    const prefetched = prefetchedRef.current.get(pageNum)
    if (prefetched) {
      setVideos(prefetched)
      prefetchedRef.current.delete(pageNum)
      setPage(pageNum)
      setLoading(false)
    } else {
      setPage(pageNum)
      loadPage(pageNum)
    }
    setTimeout(() => setPageDir(null), 50)
  }, [page, loadPage])

  const prefetchPage = useCallback(async (pageNum: number) => {
    if (prefetchedRef.current.has(pageNum)) return
    try {
      const data = await GetVideos(pageNum) as unknown as Video[]
      if (data && data.length > 0) prefetchedRef.current.set(pageNum, data)
    } catch { /* silent */ }
  }, [])

  useEffect(() => { loadPage(1); prefetchPage(2) }, [loadPage, prefetchPage])
  useEffect(() => { prefetchPage(page + 1) }, [page, prefetchPage])

  /* ── Magnets ─── */

  const fetchMagnets = useCallback(async (code: string, title: string) => {
    const key = code || ('title:' + title)
    setMagnetStates(prev => ({ ...prev, [key]: { status: 'loading' } }))
    try {
      const data = await GetMagnets(code, title) as unknown as MagnetResult
      setMagnetStates(prev => ({ ...prev, [key]: { status: 'done', data } }))
    } catch (e: any) {
      setMagnetStates(prev => ({ ...prev, [key]: { status: 'error', data: { note: String(e) } as MagnetResult } }))
    }
  }, [])

  const refreshMagnets = useCallback((code: string, title: string) => {
    const key = code || ('title:' + title)
    setMagnetStates(prev => { const n = { ...prev }; delete n[key]; return n })
    fetchMagnets(code, title)
  }, [fetchMagnets])

  /* ── Auto-fetch magnets when video is selected ─── */

  useEffect(() => {
    if (selected === null || !videos[selected]) return
    const v = videos[selected]
    const key = v.code || ('title:' + v.title)
    if (!magnetStates[key]) {
      fetchMagnets(v.code, v.title)
    }
  }, [selected, videos, magnetStates, fetchMagnets])

  const handleSelect = useCallback((idx: number) => {
    setSelected(prev => prev === idx ? null : idx)
  }, [])

  /* ── Keyboard navigation ─── */

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'ArrowLeft' && page > 1) goToPage(page - 1)
      if (e.key === 'ArrowRight') goToPage(page + 1)
      if (e.key === 'Escape') setSelected(null)
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [page, goToPage])

  /* ── Selected video ─── */

  const selVideo = selected !== null ? videos[selected] : null
  const selKey = selVideo ? (selVideo.code || ('title:' + selVideo.title)) : ''
  const selMagnet = selKey ? magnetStates[selKey] : undefined

  if (showSplash) {
    return <Splash onFinish={() => setShowSplash(false)} />
  }

  return (
    <div className="app fade-in">
      {/* Header */}
      <header>
        <div className="header-left">
          <svg className="header-logo" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <polygon points="5 3 19 12 5 21 5 3" />
          </svg>
          <h1>CosAvLink</h1>
        </div>
        <div className="header-right">
          <span className="sub">cosplay.jav.pw 最新视频 · 磁力自动从 javdb 获取</span>
        </div>
      </header>

      {/* Body: grid + detail */}
      <div className="body">
        {/* Video grid */}
        <div className="grid-wrap">
          {error && <div className="err-banner">{error}</div>}

          <div className={`grid ${pageDir === 'right' ? 'slide-in-right' : pageDir === 'left' ? 'slide-in-left' : ''}`}>
            {loading && videos.length === 0 ? (
              <div className="grid-loading">
                <div className="spinner" />
                <span>加载中…</span>
              </div>
            ) : (
              videos.map((v, i) => (
                <div
                  key={page + '-' + i}
                  className={`card card-enter ${selected === i ? 'card-selected' : ''}`}
                  style={{ animationDelay: `${i * 40}ms` }}
                  onClick={() => handleSelect(i)}
                >
                  <div className="card-cover">
                    <img loading="lazy" src={v.cover} alt="" />
                    {v.code && <span className="card-code">{v.code}</span>}
                    {!v.code && <span className="card-nocode">无番号</span>}
                  </div>
                  <div className="card-title" title={v.title}>{v.title}</div>
                </div>
              ))
            )}
          </div>
        </div>

        {/* Detail panel */}
        <div className={`detail ${selVideo ? 'detail-open' : ''}`}>
          {selVideo ? (
            <DetailPanel
              video={selVideo}
              magnetState={selMagnet}
              onRefresh={() => refreshMagnets(selVideo.code, selVideo.title)}
              onClose={() => setSelected(null)}
            />
          ) : (
            <div className="detail-empty">
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1" strokeLinecap="round" strokeLinejoin="round" opacity="0.3">
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="12" />
                <line x1="12" y1="16" x2="12.01" y2="16" />
              </svg>
              <p>点击左侧视频查看磁力</p>
            </div>
          )}
        </div>
      </div>

      {/* Footer / Pager */}
      <footer>
        <div className="pager">
          <button className="page-btn" disabled={page <= 1} onClick={() => goToPage(page - 1)}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="15 18 9 12 15 6" /></svg>
            上一页
          </button>
          <span className="page-info">第 {page} 页</span>
          <button className="page-btn" onClick={() => goToPage(page + 1)}>
            下一页
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="9 18 15 12 9 6" /></svg>
          </button>
        </div>
        <span className="footer-text">仅用于个人学习与技术研究 · 磁力来自 javdb.com</span>
      </footer>
    </div>
  )
}

/* ── Detail Panel ──────────────────────────────────────── */

function DetailPanel({ video, magnetState, onRefresh, onClose }: {
  video: Video
  magnetState?: { status: MagnetStatus; data?: MagnetResult }
  onRefresh: () => void
  onClose: () => void
}) {
  const status = magnetState?.status || 'waiting'
  const data = magnetState?.data

  return (
    <div className="detail-inner">
      <div className="detail-header">
        <div className="detail-title" title={video.title}>{video.title}</div>
        <button className="detail-close" onClick={onClose} title="关闭">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
        </button>
      </div>

      {video.code && <div className="detail-code">{video.code}</div>}

      <a className="detail-link" href={video.detailUrl} target="_blank" rel="noopener noreferrer">
        在 cosplay.jav.pw 查看 ↗
      </a>

      <div className="detail-actions">
        <button className="btn btn-primary" onClick={onRefresh} disabled={status === 'loading'}>
          {status === 'loading' ? '查询中…' : '刷新磁力'}
        </button>
      </div>

      <div className="detail-magnets">
        {status === 'loading' && (
          <div className="magnet-loading">
            <div className="spinner small" />
            <span>正在查询 javdb…</span>
          </div>
        )}

        {status === 'done' && data && <MagnetResultView data={data} />}

        {status === 'error' && data && (
          <div className="magnet-warn">{data.note || '查询失败'}</div>
        )}
      </div>
    </div>
  )
}

/* ── Magnet Result ─────────────────────────────────────── */

function MagnetResultView({ data }: { data: MagnetResult }) {
  if (data.blocked) {
    return <div className="magnet-warn">{data.note || '被 Cloudflare 拦截，请稍后重试'}</div>
  }

  if (!data.magnets || data.magnets.length === 0) {
    return (
      <div className="magnet-empty">
        <div className="magnet-warn">{data.note || '暂无磁力'}</div>
        {data.detailUrl && (
          <a className="detail-link" href={data.detailUrl} target="_blank" rel="noopener noreferrer">
            在 javdb 查看 ↗
          </a>
        )}
      </div>
    )
  }

  return (
    <div className="magnet-list">
      {data.detailUrl && (
        <a className="detail-link" href={data.detailUrl} target="_blank" rel="noopener noreferrer">
          在 javdb 查看 ↗
        </a>
      )}
      {data.magnets.map((m, i) => (
        <MagnetItem key={i} magnet={m} delay={i * 60} />
      ))}
    </div>
  )
}

/* ── Magnet Item ───────────────────────────────────────── */

function MagnetItem({ magnet, delay }: { magnet: Magnet; delay: number }) {
  const [copied, setCopied] = useState(false)

  const copyLink = () => {
    navigator.clipboard.writeText(magnet.link || '').then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }

  return (
    <div className="magnet-item card-enter" style={{ animationDelay: `${delay}ms` }}>
      <div className="magnet-name">{magnet.name || 'magnet'}</div>
      <div className="magnet-meta">
        {magnet.size && <span className="magnet-tag">{magnet.size}</span>}
        {magnet.date && <span className="magnet-tag">{magnet.date}</span>}
        {(magnet.tags || []).map((t, i) => <span key={i} className="magnet-tag">{t}</span>)}
        {magnet.source === 'comment' && <span className="magnet-tag tag-comment">短评</span>}
      </div>
      <div className="magnet-actions">
        <a className="btn btn-sm btn-primary" href={magnet.link}>打开磁力</a>
        <button className="btn btn-sm btn-ghost" onClick={copyLink}>{copied ? '✓ 已复制' : '复制'}</button>
      </div>
    </div>
  )
}

export default App
