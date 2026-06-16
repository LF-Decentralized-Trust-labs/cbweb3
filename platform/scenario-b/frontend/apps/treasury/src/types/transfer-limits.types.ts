export type TransferLimit = {
  limit_id: string;
  central_bank_id: string;
  participant_id: string;
  currency: string;
  max_amount: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
};

export type CreateTransferLimitPayload = {
  participant_id?: string;
  currency?: string;
  max_amount: string;
};
