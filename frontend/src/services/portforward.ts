// Lightweight service wrapping Wails SSHBridge port-forward APIs

export type ForwardMode = 'local' | 'remote' | 'dynamic'
export interface ForwardItem {
  id: string
  mode: ForwardMode
  from: string
  to: string
}

function bridge(): any {
  const b = (window as any)?.go?.main?.SSHBridge
  if (!b) throw new Error('SSHBridge not available. Rebuild wails bindings.')
  return b
}

export async function startLocalForward(sessionId: string, localAddr: string, remoteAddr: string): Promise<string> {
  return await bridge().StartLocalForward(sessionId, localAddr, remoteAddr)
}

export async function startRemoteForward(sessionId: string, remoteBind: string, localTarget: string): Promise<string> {
  return await bridge().StartRemoteForward(sessionId, remoteBind, localTarget)
}

export async function startDynamicForward(sessionId: string, localSocks: string): Promise<string> {
  return await bridge().StartDynamicForward(sessionId, localSocks)
}

export async function listForwards(sessionId: string): Promise<ForwardItem[]> {
  const s = await bridge().ListForwards(sessionId)
  if (!s) return []
  try {
    const raw = JSON.parse(s) as any[]
    if (!Array.isArray(raw)) return []
    return raw.map((it: any) => {
      const id = it?.id ?? it?.ID ?? ''
      const modeRaw = it?.mode ?? it?.Mode ?? ''
      const mode = String(modeRaw).toLowerCase() as ForwardMode
      const from = it?.from ?? it?.From ?? ''
      const to = it?.to ?? it?.To ?? ''
      return { id, mode, from, to }
    }).filter(x => x.id)
  } catch {
    return []
  }
}

export async function stopForward(sessionId: string, id: string): Promise<string> {
  return await bridge().StopForward(sessionId, id)
}

export async function stopAllForwards(sessionId: string): Promise<string> {
  return await bridge().StopAllForwards(sessionId)
}
