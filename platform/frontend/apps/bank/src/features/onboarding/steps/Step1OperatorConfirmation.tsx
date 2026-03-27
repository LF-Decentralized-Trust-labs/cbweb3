import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle } from "@cbweb3/ui";

type Step1OperatorConfirmationProps = {
  operatorId: string;
  onStart: () => void;
};

export function Step1OperatorConfirmation({ operatorId, onStart }: Step1OperatorConfirmationProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Step 1: Operator Confirmation</CardTitle>
        <CardDescription>Confirm your identity before initiating bank onboarding.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-muted-foreground">
          You are signed in as <span className="font-medium text-foreground">{operatorId}</span>. Continue to register your institution with the
          Central Bank.
        </p>
        <Button onClick={onStart}>Start Onboarding</Button>
      </CardContent>
    </Card>
  );
}