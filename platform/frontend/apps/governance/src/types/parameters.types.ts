export type GovernanceParameters = {
  txLimitMin: number;
  txLimitMax: number;
  slippageTolerance: number;
  settlementWindowSeconds: number;
};

export type ParameterUpdate = {
  field: keyof GovernanceParameters;
  previousValue: number;
  proposedValue: number;
};

export type UpdateParametersPayload = {
  txLimitMin: number;
  txLimitMax: number;
  slippageTolerance: number;
  settlementWindowSeconds: number;
  reason: string;
};
