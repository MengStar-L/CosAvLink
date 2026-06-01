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

function App() {
  const [page, setPage] = useState(1)
  const [videos, setVideos] = useState<Video[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Magnet state per card (keyed by cacheKey)
  const [magnetStates, setMagnetStates] = useState<Record<string, { status: MagnetStatus; data?: MagnetResult }>>({})

  // Prefetched pages cache
  const prefetchedRef = useRef<Map<number, Video[]>>(new Map())

  // Observer ref
  const observerRef = useRef<IntersectionObserver | null>(null)
  const cardsRef = useRef<HTMLDivElement>(null)

  // Load videos for a page
  const loadPage = useCallback(async (pageNum: number) => {
    setLoading(true)
    setError('')
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

  // Navigate to page (use prefetch cache if available)
  const goToPage = useCallback((pageNum: number) => {
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
  }, [loadPage])

  // Prefetch next page
  const prefetchPage = useCallback(async (pageNum: number) => {
    if (prefetchedRef.current.has(pageNum)) return
    try {
      const data = await GetVideos(pageNum) as unknown as Video[]
      if (data && data.length > 0) {
        prefetchedRef.current.set(pageNum, data)
      }
    } catch {
      // silent
    }
  }, [])

  // Initial load + prefetch page 2
  useEffect(() => {
    loadPage(1)
    prefetchPage(2)
  }, [loadPage, prefetchPage])

  // Prefetch next page when page changes
  useEffect(() => {
    prefetchPage(page + 1)
  }, [page, prefetchPage])

  // Fetch magnets for a card
  const fetchMagnets = useCallback(async (code: string, title: string) => {
    const cacheKey = code || ('title:' + title)
    setMagnetStates(prev => ({ ...prev, [cacheKey]: { status: 'loading' } }))
    try {
      const data = await GetMagnets(code, title) as unknown as MagnetResult
      setMagnetStates(prev => ({ ...prev, [cacheKey]: { status: 'done', data } }))
    } catch (e: any) {
      setMagnetStates(prev => ({ ...prev, [cacheKey]: { status: 'error', data: { note: String(e) } as MagnetResult } }))
    }
  }, [])

  // Refresh magnets (clear cache and re-fetch)
  const refreshMagnets = useCallback((code: string, title: string) => {
    const cacheKey = code || ('title:' + title)
    setMagnetStates(prev => {
      const next = { ...prev }
      delete next[cacheKey]
      return next
    })
    fetchMagnets(code, title)
  }, [fetchMagnets])

  // IntersectionObserver for auto magnet prefetch
  useEffect(() => {
    if (observerRef.current) observerRef.current.disconnect()

    observerRef.current = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            const card = entry.target as HTMLElement
            observerRef.current?.unobserve(card)
            const code = card.dataset.code || ''
            const title = card.dataset.title || ''
            const cacheKey = code || ('title:' + title)
            if (!magnetStates[cacheKey]) {
              fetchMagnets(code, title)
            }
          }
        })
      },
      { rootMargin: '200px' }
    )

    // Observe all cards
    const timer = setTimeout(() => {
      const container = cardsRef.current
      if (!container) return
      container.querySelectorAll('.card[data-title]').forEach((card) => {
        observerRef.current?.observe(card)
      })
    }, 100)

    return () => {
      clearTimeout(timer)
      observerRef.current?.disconnect()
    }
  }, [videos, magnetStates, fetchMagnets])

  return (
    <>
      <header>
        <h1>CosAvLink</h1>
        <span className="sub">cosplay.jav.pw 最新视频 · 磁力自动从 javdb 获取</span>
      </header>
      <main>
        {error && <div className="err-banner">{error}</div>}
        <div className="grid" ref={cardsRef}>
          {loading && videos.length === 0 ? (
            <div style={{ gridColumn: '1/-1', textAlign: 'center', color: '#8a94a6', padding: '40px 0' }}>
              加载中…
            </div>
          ) : (
            videos.map((v, i) => (
              <VideoCard
                key={page + '-' + i}
                video={v}
                magnetState={magnetStates[v.code || ('title:' + v.title)]}
                onFetch={() => fetchMagnets(v.code, v.title)}
                onRefresh={() => refreshMagnets(v.code, v.title)}
              />
            ))
          )}
        </div>
        <div className="pager">
          <button disabled={page <= 1} onClick={() => goToPage(page - 1)}>上一页</button>
          <span className="page-info">第 {page} 页</span>
          <button onClick={() => goToPage(page + 1)}>下一页</button>
        </div>
      </main>
      <footer>
        仅用于个人学习与技术研究。磁力链接来自第三方网站 javdb.com，资源与版权由对应站点负责。
      </footer>
    </>
  )
}

// VideoCard component
function VideoCard({ video, magnetState, onFetch, onRefresh }: {
  video: Video
  magnetState?: { status: MagnetStatus; data?: MagnetResult }
  onFetch: () => void
  onRefresh: () => void
}) {
  const status = magnetState?.status || 'waiting'
  const data = magnetState?.data

  const btnClass = status === 'waiting' || status === 'loading' ? 'btn prefetch' : 'btn'
  const btnDisabled = status === 'loading'
  const btnText = status === 'waiting' ? '等待中…'
    : status === 'loading' ? '加载中…'
    : '刷新磁力'
  const btnAction = status === 'done' || status === 'error' ? onRefresh : undefined

  return (
    <div className="card" data-code={video.code} data-title={video.title}>
      <a className="cover" href={video.detailUrl} target="_blank" rel="noopener noreferrer">
        <img loading="lazy" src={video.cover} alt="" />
      </a>
      <div className="body">
        <div className="title" title={video.title}>{video.title}</div>
        <div className="meta">
          {video.code ? (
            <span className="code">{video.code}</span>
          ) : (
            <span className="nocode">无番号</span>
          )}
        </div>
        <button
          className={btnClass}
          disabled={btnDisabled}
          onClick={btnAction}
        >
          {btnText}
        </button>
        {(status === 'loading' || status === 'done' || status === 'error') && (
          <MagnetBox status={status} data={data} />
        )}
      </div>
    </div>
  )
}

// MagnetBox component
function MagnetBox({ status, data }: { status: MagnetStatus; data?: MagnetResult }) {
  if (status === 'loading') {
    return <div className="magnets"><div className="loading">正在查询 javdb…</div></div>
  }
  if (!data) return null

  const detailLink = data.detailUrl
    ? <a className="detail" target="_blank" rel="noopener noreferrer" href={data.detailUrl}>在 javdb 查看 ↗</a>
    : null

  if (data.blocked) {
    return (
      <div className="magnets">
        <div className="warn">{data.note || '被 Cloudflare 拦截，请稍后重试'}</div>
      </div>
    )
  }

  if (!data.magnets || data.magnets.length === 0) {
    return (
      <div className="magnets">
        <div className="warn">{data.note || '暂无磁力'}</div>
        {detailLink}
      </div>
    )
  }

  return (
    <div className="magnets">
      {detailLink}
      <ul className="mlist">
        {data.magnets.map((m, i) => (
          <MagnetItem key={i} magnet={m} />
        ))}
      </ul>
    </div>
  )
}

// MagnetItem component
function MagnetItem({ magnet }: { magnet: Magnet }) {
  const [copied, setCopied] = useState(false)

  const copyLink = () => {
    navigator.clipboard.writeText(magnet.link || '').then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }

  return (
    <li>
      <div className="mname">{magnet.name || 'magnet'}</div>
      <div className="mmeta">
        {magnet.size && <span>{magnet.size}</span>}
        {magnet.date && <span>{magnet.date}</span>}
        {(magnet.tags || []).map((t, i) => <span key={i} className="t">{t}</span>)}
        {magnet.source === 'comment' && <span className="src">短评</span>}
      </div>
      <div className="mact">
        <a className="open" href={magnet.link}>打开磁力</a>
        <button className="copy" onClick={copyLink}>{copied ? '已复制' : '复制'}</button>
      </div>
    </li>
  )
}

export default App
