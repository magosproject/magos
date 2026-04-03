export type UserRole = "admin" | "user";

export interface User {
  name: string;
  email: string;
  role: UserRole;
  projectIds: string[];
}

export const currentUser: User = {
  name: "Ramon",
  email: "ramon@magos.dev",
  role: "admin", // Change to "user" to test user role
  projectIds: ["p-0001", "p-0002"],
};
