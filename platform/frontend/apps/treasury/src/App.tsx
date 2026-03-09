import { Button } from '@cbweb3/ui'

function App() {
  return (
    <main className="mx-auto flex min-h-screen max-w-4xl flex-col items-center justify-center gap-6 p-6">
      <h1 className="text-3xl font-semibold tracking-tight">CBWeb3 Treasury App</h1>
      <p className="text-muted-foreground">Shared shadcn/ui components from @cbweb3/ui</p>
      <div className="flex items-center gap-3">
        <Button>Primary Action</Button>
        <Button variant="outline">Secondary Action</Button>
      </div>
    </main>
  )
}

export default App
