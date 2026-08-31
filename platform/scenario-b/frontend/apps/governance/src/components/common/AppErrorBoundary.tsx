// SPDX-License-Identifier: Apache-2.0

import { Button } from "@cbweb3/ui";
import type { ReactNode } from "react";
import { Component } from "react";

type AppErrorBoundaryProps = {
  children: ReactNode;
};

type AppErrorBoundaryState = {
  hasError: boolean;
};

export class AppErrorBoundary extends Component<AppErrorBoundaryProps, AppErrorBoundaryState> {
  state: AppErrorBoundaryState = {
    hasError: false,
  };

  static getDerivedStateFromError(): AppErrorBoundaryState {
    return { hasError: true };
  }

  render() {
    if (this.state.hasError) {
      return (
        <main className="flex min-h-screen items-center justify-center p-6">
          <div className="space-y-3 text-center">
            <h1 className="text-2xl font-semibold">Something went wrong</h1>
            <p className="text-sm text-muted-foreground">Governance Portal encountered an unexpected error.</p>
            <Button onClick={() => window.location.reload()}>Reload</Button>
          </div>
        </main>
      );
    }
    return this.props.children;
  }
}
