// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Label,
  Separator,
  toast,
  BackToLauncherButton,
} from "@cbweb3/ui";
import { Building2, LockKeyhole, ShieldCheck, Coins } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useNavigate } from "react-router-dom";
import { useAuthStore } from "../stores";

const schema = z.object({
  clientId: z.string().min(3, "Username must be at least 3 characters"),
  clientSecret: z.string().min(6, "Password must be at least 6 characters"),
});

type LoginForm = z.infer<typeof schema>;

const defaultClientId = (import.meta.env.VITE_KC_CLIENT_ID ?? "").trim();
const defaultClientSecret = (import.meta.env.VITE_KC_CLIENT_SECRET ?? "").trim();

export function LoginPage() {
  const navigate = useNavigate();
  const { login, status, error, isAuthenticated } = useAuthStore();
  const form = useForm<LoginForm>({
    resolver: zodResolver(schema),
    defaultValues: {
      clientId: defaultClientId,
      clientSecret: defaultClientSecret,
    },
  });

  useEffect(() => {
    if (isAuthenticated) {
      toast("Signed in", {
        description: "Welcome to the Treasury Portal.",
      });
      navigate("/", { replace: true });
    }
  }, [isAuthenticated, navigate]);

  const onSubmit = form.handleSubmit(async (values) => {
    await login(values.clientId, values.clientSecret);
  });

  return (
    <main className="min-h-screen bg-gradient-to-b from-primary/10 via-background to-background">
      <div className="mx-auto grid min-h-screen max-w-6xl gap-8 px-4 py-8 lg:grid-cols-2 lg:items-center">
        <section className="hidden rounded-2xl border border-border/70 bg-card/70 p-8 backdrop-blur lg:block">
          <Badge variant="secondary" className="mb-4 w-fit">
            LNET · Treasury
          </Badge>
          <h1 className="text-3xl font-semibold tracking-tight">Central Bank Treasury Portal</h1>
          <p className="mt-3 max-w-md text-sm text-muted-foreground">
            Manage tCeBM issuance, redemption, funding approvals, and reserve reconciliation with strict policy controls.
          </p>

          <div className="mt-8 grid gap-4">
            <div className="flex items-start gap-3 rounded-lg border border-border/80 bg-background/80 p-4">
              <ShieldCheck className="mt-0.5 h-5 w-5 text-primary" />
              <div>
                <p className="text-sm font-medium">Privileged operations</p>
                <p className="text-xs text-muted-foreground">Supply-changing actions are restricted to Treasury role claims.</p>
              </div>
            </div>
            <div className="flex items-start gap-3 rounded-lg border border-border/80 bg-background/80 p-4">
              <Coins className="mt-0.5 h-5 w-5 text-primary" />
              <div>
                <p className="text-sm font-medium">Burn-to-mint enforcement</p>
                <p className="text-xs text-muted-foreground">Minting requires approved requests and validated reserve backing.</p>
              </div>
            </div>
            <div className="flex items-start gap-3 rounded-lg border border-border/80 bg-background/80 p-4">
              <LockKeyhole className="mt-0.5 h-5 w-5 text-primary" />
              <div>
                <p className="text-sm font-medium">Secure session model</p>
                <p className="text-xs text-muted-foreground">No sensitive data persistence outside active runtime state.</p>
              </div>
            </div>
          </div>
        </section>

        <Card className="mx-auto w-full max-w-md border-border/80 shadow-lg">
          <CardHeader>
            <BackToLauncherButton className="-ml-2 mb-2 self-start" />
            <div className="mb-2 flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Building2 className="h-5 w-5" />
            </div>
            <CardTitle>Sign in to Treasury Portal</CardTitle>
            <CardDescription>Use institutional credentials to access treasury operations.</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={onSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="clientId">Username</Label>
                <Input id="clientId" {...form.register("clientId")} autoComplete="username" />
                {form.formState.errors.clientId ? <p className="text-xs text-destructive">{form.formState.errors.clientId.message}</p> : null}
              </div>

              <div className="space-y-2">
                <Label htmlFor="clientSecret">Password</Label>
                <Input id="clientSecret" type="password" {...form.register("clientSecret")} autoComplete="current-password" />
                {form.formState.errors.clientSecret ? <p className="text-xs text-destructive">{form.formState.errors.clientSecret.message}</p> : null}
              </div>

              {error ? <p className="text-sm text-destructive">{error}</p> : null}

              <Button type="submit" className="w-full" disabled={status === "loading"}>
                {status === "loading" ? "Signing in..." : "Sign in"}
              </Button>
            </form>

            <Separator className="my-4" />

            <p className="text-center text-xs text-muted-foreground">Protected environment · RBAC enforced · Treasury-grade controls</p>
          </CardContent>
        </Card>
      </div>
    </main>
  );
}
