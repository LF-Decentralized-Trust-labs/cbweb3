import { z } from "zod";

export const institutionSchema = z.object({
  institution_name: z.string().min(2, "Institution name must have at least 2 characters"),
  bank_code: z
    .string()
    .min(1, "Bank code is required")
    .max(8, "Bank code must contain up to 8 characters")
    .regex(/^[a-z0-9-]+$/, "Use lowercase letters, numbers, or hyphen"),
  country: z.string().length(2, "Country must be 2 characters (ISO 3166-1 alpha-2)"),
  role: z.enum(["ROLE_COMMERCIAL_BANK", "ROLE_TREASURY_BANK"]),
  email: z.string().email("Enter a valid email"),
  username: z
    .string()
    .min(3, "Username must have at least 3 characters")
    .regex(/^[a-z0-9_-]+$/, "Use lowercase letters, numbers, underscore, or hyphen"),
});

export type InstitutionFormValues = z.infer<typeof institutionSchema>;