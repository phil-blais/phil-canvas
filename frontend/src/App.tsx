import { useMemo, useState } from 'react'
import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import { CollaborativeCanvas } from './collab/CollaborativeCanvas'
import { useSession, type Session } from './session/useSession'
import { FlyoutPanel } from './ui/FlyoutPanel'
import { PublishedScene } from './ui/PublishedScene'
import { DocumentsPanel } from './ui/DocumentsPanel'
import { DocumentControls } from './ui/DocumentControls'
import { resolveUsername } from './collab/identity'
import { resolveWsBaseUrl } from './collab/transport'

const wsBaseUrl = resolveWsBaseUrl()

function App() {
  const session = useSession()

  if (!session.ready) {
    return <div className="landing-backdrop">Loading…</div>
  }

  return session.room ? <RoomView session={session} /> : <LandingView session={session} />
}

function LandingView({ session }: { session: Session }) {
  return (
    <>
      {/* The public site: published scene rendered read-only. */}
      <PublishedScene />
      <FlyoutPanel>
        <DocumentsPanel session={session} />
      </FlyoutPanel>
    </>
  )
}

function RoomView({ session }: { session: Session }) {
  const room = session.room!
  const [excalidrawApi, setExcalidrawApi] = useState<ExcalidrawImperativeAPI | null>(null)

  // Stable identity for the lifetime of this room membership.
  const user = useMemo(
    () => ({ username: resolveUsername(room.role, session.admin?.user) }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [room.id],
  )

  return (
    <>
      <CollaborativeCanvas
        roomId={room.id}
        wsBaseUrl={wsBaseUrl}
        getToken={() => room.token}
        user={user}
        seed={room.seed}
        onRoomClosed={session.leaveRoom}
        onAuthError={session.leaveRoom}
        onSeedConsumed={session.clearSeed}
        onApiChange={setExcalidrawApi}
      />
      <FlyoutPanel>
        <DocumentControls
          room={room}
          admin={session.admin}
          markSceneId={session.markSceneId}
          closeRoom={session.closeRoom}
          leaveRoom={session.leaveRoom}
          renameDocument={session.renameDocument}
          excalidrawApi={excalidrawApi}
          username={user.username}
        />
      </FlyoutPanel>
    </>
  )
}

export default App
