// SPDX-License-Identifier: Apache-2.0

import { Permission } from "../types";
import { useAuthStore } from "../stores";

export const usePermissions = () => {
  const permissions = useAuthStore((state) => state.user?.permissions ?? []);

  return {
    permissions,
    hasPermission: (permission: Permission) => permissions.includes(permission),
  };
};
