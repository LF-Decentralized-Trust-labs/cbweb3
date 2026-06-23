// SPDX-License-Identifier: Apache-2.0

import { zodResolver } from "@hookform/resolvers/zod";
import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Label,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@cbweb3/ui";
import { Controller, useForm } from "react-hook-form";
import type { InstitutionFormValues } from "../schemas/onboarding.schema";
import { institutionSchema } from "../schemas/onboarding.schema";

type Step2InstitutionFormProps = {
  loading: boolean;
  error: string | null;
  onSubmit: (values: InstitutionFormValues) => Promise<void>;
};

export function Step2InstitutionForm({ loading, error, onSubmit }: Step2InstitutionFormProps) {
  const form = useForm<InstitutionFormValues>({
    resolver: zodResolver(institutionSchema),
    defaultValues: {
      institution_name: "",
      bank_code: "",
      country: "BR",
      role: "ROLE_COMMERCIAL_BANK",
      email: "",
      username: "",
    },
  });

  const submit = form.handleSubmit(async (values) => {
    await onSubmit(values);
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>Step 2: Institution Data</CardTitle>
        <CardDescription>Submit your institution details to open the onboarding request.</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2 md:col-span-2">
            <Label htmlFor="institution_name">Institution Name</Label>
            <Input id="institution_name" {...form.register("institution_name")} />
            {form.formState.errors.institution_name ? (
              <p className="text-xs text-destructive">{form.formState.errors.institution_name.message}</p>
            ) : null}
          </div>

          <div className="space-y-2">
            <Label htmlFor="bank_code">Bank Code</Label>
            <Input id="bank_code" {...form.register("bank_code")} />
            {form.formState.errors.bank_code ? <p className="text-xs text-destructive">{form.formState.errors.bank_code.message}</p> : null}
          </div>

          <div className="space-y-2">
            <Label htmlFor="country">Country</Label>
            <Input id="country" maxLength={2} {...form.register("country")} />
            {form.formState.errors.country ? <p className="text-xs text-destructive">{form.formState.errors.country.message}</p> : null}
          </div>

          <div className="space-y-2">
            <Label>Role</Label>
            <Controller
              control={form.control}
              name="role"
              render={({ field }) => (
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select role" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="ROLE_COMMERCIAL_BANK">Commercial Bank</SelectItem>
                    <SelectItem value="ROLE_TREASURY_BANK">Treasury Bank</SelectItem>
                  </SelectContent>
                </Select>
              )}
            />
            {form.formState.errors.role ? <p className="text-xs text-destructive">{form.formState.errors.role.message}</p> : null}
          </div>

          <div className="space-y-2">
            <Label htmlFor="username">Username</Label>
            <Input id="username" {...form.register("username")} />
            {form.formState.errors.username ? <p className="text-xs text-destructive">{form.formState.errors.username.message}</p> : null}
          </div>

          <div className="space-y-2 md:col-span-2">
            <Label htmlFor="email">Email</Label>
            <Input id="email" type="email" {...form.register("email")} />
            {form.formState.errors.email ? <p className="text-xs text-destructive">{form.formState.errors.email.message}</p> : null}
          </div>

          {error ? <p className="md:col-span-2 text-sm text-destructive">{error}</p> : null}

          <div className="md:col-span-2">
            <Button type="submit" disabled={loading}>
              {loading ? "Submitting..." : "Submit Onboarding Request"}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}