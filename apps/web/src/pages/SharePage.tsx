import { useEffect, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { Shares, type DocNode } from '@/lib/api'
import { useDocsStore } from '@/store/docs'

export default function SharePage() {
  const { token } = useParams<{ token: string }>()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const upsertFromServer = useDocsStore((s) => s.upsertFromServer)
  const [doc, setDoc] = useState<DocNode | null>(null)
  const [error, setError] = useState<string | null>(null)

  // fullscreen 参数：跳转到主站时带上，让外壳隐藏顶部和左侧菜单（iframe 嵌套不变）
  const fullscreen = searchParams.get('fullscreen') !== null
    && searchParams.get('fullscreen') !== '0'
    && searchParams.get('fullscreen') !== 'false'

  useEffect(() => {
    if (!token) return
    console.debug('[web-doc share] load share token', {
      token,
      pathname: location.pathname,
      search: location.search,
      fullscreen,
    })
    Shares.info(token)
      .then((r) => {
        console.debug('[web-doc share] share token resolved', {
          token,
          docId: r.doc.id,
          title: r.doc.title,
          visibility: r.doc.visibility,
          fullscreen,
        })
        setDoc(r.doc)
      })
      .catch((err) => {
        console.warn('[web-doc share] share token lookup failed', {
          token,
          status: err?.response?.status,
          message: err?.message,
        })
        setError('链接无效或已过期')
      })
  }, [token, fullscreen])

  // 始终跳转到主站文档页（/v/:docId），保持完整 React 外壳 + iframe 的双层结构。
  // - 默认：显示顶部 + 左侧菜单
  // - 带 ?fullscreen：隐藏顶部 + 左侧菜单（iframe 仍然嵌套，路由不被破坏）
  // 跳转前先把文档 upsert 到 store，避免 HomePage 因为本地 nodes 里没有它而
  // 误判为「不存在」并重定向回首页（典型场景：未登录访客打开分享链接）。
  // 同时把 share token 带进目标 URL（?share=），这样访客刷新 /v/ 页面后
  // HomePage 仍能凭它重新解析分享、恢复文档，而不是要求登录。
  useEffect(() => {
    if (!doc || !token) return
    console.debug('[web-doc share] upsert shared doc before redirect', {
      docId: doc.id,
      title: doc.title,
      fullscreen,
    })
    upsertFromServer(doc, { shared: true, select: true })
    const query = new URLSearchParams({ share: token })
    if (fullscreen) query.set('fullscreen', '1')
    const target = `/v/${doc.id}?${query.toString()}`
    console.debug('[web-doc share] navigate to shared doc', {
      from: location.pathname + location.search,
      target,
      replace: true,
    })
    navigate(target, { replace: true })
  }, [doc, token, fullscreen, navigate, upsertFromServer])

  if (error) {
    return (
      <div className="h-full w-full flex items-center justify-center gradient-bg">
        <div className="glass border border-border/60 rounded-xl px-8 py-6 text-center">
          <h1 className="text-lg font-semibold mb-2">😕 {error}</h1>
          <p className="text-sm text-muted-foreground">请联系分享者获取新链接。</p>
        </div>
      </div>
    )
  }
  return (
    <div className="h-full w-full flex items-center justify-center text-muted-foreground text-sm">
      {doc ? '正在跳转…' : '加载中…'}
    </div>
  )
}
