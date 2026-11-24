export type AdminUser = {
  id: number;
  username: string;
  email: string;
  display_name?: string;
  status: string;
  roles: string[];
};

export type AuthResponse = {
  access_token: string;
  expires_at: string;
  user: AdminUser;
  roles: string[];
};

export type Role = {
  id: number;
  code: string;
  name: string;
  description?: string;
  status: string;
};
