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
} from "@cbweb3/ui";
import { ArrowRight, Building2, Globe, LockKeyhole, ShieldCheck } from "lucide-react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { resolvePortal } from "../utils/routing";

const schema = z.object({
  clientId: z.string().min(3, "Client ID must be at least 3 characters"),
});

type DispatcherForm = z.infer<typeof schema>;

export function DispatcherPage() {
  const form = useForm<DispatcherForm>({
    resolver: zodResolver(schema),
    defaultValues: { clientId: "" },
  });

  const onSubmit = form.handleSubmit((values) => {
    const portal = resolvePortal(values.clientId);
    if (!portal) {
      form.setError("clientId", {
        message: "Institution not identified. Please check your Client ID.",
      });
      return;
    }
    window.location.href = `${portal.portalUrl}/login?username=${encodeURIComponent(values.clientId)}`;
  });

  return (
    <main className="min-h-screen bg-gradient-to-b from-primary/10 via-background to-background">
      <div className="mx-auto grid min-h-screen max-w-6xl gap-8 px-4 py-8 lg:grid-cols-2 lg:items-center">
        <section className="hidden rounded-2xl border border-border/70 bg-card/70 p-8 backdrop-blur lg:block">
          <Badge variant="secondary" className="mb-4 w-fit">
            LNET · CBWeb3
          </Badge>
          <h1 className="text-3xl font-semibold tracking-tight">
            Unified Portal Access
          </h1>
          <p className="mt-3 max-w-md text-sm text-muted-foreground">
            Enter your Client ID to be directed to the correct institutional portal
            for your organization.
          </p>

          <div className="mt-8 grid gap-4">
            <div className="flex items-start gap-3 rounded-lg border border-border/80 bg-background/80 p-4">
              <Globe className="mt-0.5 h-5 w-5 text-primary" />
              <div>
                <p className="text-sm font-medium">Single entry point</p>
                <p className="text-xs text-muted-foreground">
                  One URL for all institutions — automatically routed to your portal.
                </p>
              </div>
            </div>
            <div className="flex items-start gap-3 rounded-lg border border-border/80 bg-background/80 p-4">
              <ShieldCheck className="mt-0.5 h-5 w-5 text-primary" />
              <div>
                <p className="text-sm font-medium">Institution-bound access</p>
                <p className="text-xs text-muted-foreground">
                  Credentials are verified against your institution's auth service.
                </p>
              </div>
            </div>
            <div className="flex items-start gap-3 rounded-lg border border-border/80 bg-background/80 p-4">
              <LockKeyhole className="mt-0.5 h-5 w-5 text-primary" />
              <div>
                <p className="text-sm font-medium">Secure by design</p>
                <p className="text-xs text-muted-foreground">
                  Authentication happens on your institution's dedicated portal.
                </p>
              </div>
            </div>
          </div>
        </section>

        <Card className="mx-auto w-full max-w-md border-border/80 shadow-lg">
          <CardHeader>
            <div className="mb-2 flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Building2 className="h-5 w-5" />
            </div>
            <CardTitle>Access your portal</CardTitle>
            <CardDescription>
              Enter your Client ID to be redirected to your institution's login page.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={onSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="clientId">Client ID</Label>
                <Input
                  id="clientId"
                  {...form.register("clientId")}
                  autoComplete="username"
                  placeholder="e.g. bank-a-client"
                />
                {form.formState.errors.clientId ? (
                  <p className="text-xs text-destructive">
                    {form.formState.errors.clientId.message}
                  </p>
                ) : null}
              </div>

              <Button type="submit" className="w-full">
                Continue
                <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            </form>

            <Separator className="my-4" />

            <p className="text-center text-xs text-muted-foreground">
              LACNet · CBWeb3 · Cross-border CBDC infrastructure
            </p>
          </CardContent>
        </Card>
      </div>
    </main>
  );
}
