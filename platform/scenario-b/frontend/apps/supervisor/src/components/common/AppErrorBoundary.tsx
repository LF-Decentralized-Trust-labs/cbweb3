import React from "react";

type Props = React.PropsWithChildren;
type State = { hasError: boolean };

export class AppErrorBoundary extends React.Component<Props, State> {
  public state: State = { hasError: false };

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: unknown) {
    console.error("Supervisor portal runtime error", error);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="mx-auto flex min-h-screen max-w-xl items-center justify-center p-6 text-center">
          <div>
            <h1 className="text-xl font-semibold">Supervisor portal encountered an error</h1>
            <p className="mt-2 text-sm text-muted-foreground">Refresh the page to retry your session.</p>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
