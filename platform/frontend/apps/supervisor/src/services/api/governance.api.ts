import { mockDb } from "../mocks/mock-db";

export const governanceApi = {
  listParticipants: () => mockDb.listParticipants(),
};
