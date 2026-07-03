import { create } from 'zustand'
import { Auth, getToken, setToken, type AuthUser } from '@/lib/api'

interface AuthState {
  user: AuthUser | null
  token: string | null
  loading: boolean
  /** bootstrap() 是否已完成（无论成败）；用于区分「未登录」和「登录态还没校验完」 */
  bootstrapped: boolean
  registerEnabled: boolean
  loginOpen: boolean
  loginMode: 'login' | 'register'

  bootstrap: () => Promise<void>
  openLogin: (mode?: 'login' | 'register') => void
  closeLogin: () => void
  login: (username: string, password: string) => Promise<void>
  register: (p: { username: string; password: string; email?: string; displayName?: string }) => Promise<void>
  logout: () => void
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  token: getToken(),
  loading: false,
  bootstrapped: false,
  registerEnabled: true,
  loginOpen: false,
  loginMode: 'login',

  bootstrap: async () => {
    try {
      const info = await Auth.publicInfo()
      set({ registerEnabled: info.registerEnabled })
    } catch {/* ignore */}
    try {
      if (getToken()) {
        const u = await Auth.me()
        set({ user: u })
      }
    } catch {
      setToken(null)
      set({ user: null, token: null })
    } finally {
      set({ bootstrapped: true })
    }
  },

  openLogin: (mode = 'login') => set({ loginOpen: true, loginMode: mode }),
  closeLogin: () => set({ loginOpen: false }),

  login: async (username, password) => {
    set({ loading: true })
    try {
      const { user, token } = await Auth.login({ username, password })
      setToken(token)
      set({ user, token, loginOpen: false })
    } finally {
      set({ loading: false })
    }
  },

  register: async (p) => {
    set({ loading: true })
    try {
      const { user, token } = await Auth.register(p)
      setToken(token)
      set({ user, token, loginOpen: false })
    } finally {
      set({ loading: false })
    }
  },

  logout: () => {
    setToken(null)
    set({ user: null, token: null })
  },
}))

// 全局 401 监听：无论是登录过期还是未登录直接触发写操作，都弹出登录框
if (typeof window !== 'undefined') {
  window.addEventListener('webdoc:unauthorized', () => {
    useAuthStore.setState({ user: null, token: null })
    useAuthStore.getState().openLogin('login')
  })
}
