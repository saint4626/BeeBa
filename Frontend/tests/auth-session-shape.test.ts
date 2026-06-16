import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const authAPI = readFileSync("src/lib/api/auth.ts", "utf8");
const session = readFileSync("src/lib/auth/session.ts", "utf8");
const types = readFileSync("src/lib/api/types.ts", "utf8");

assert.match(authAPI, /import type \{ AuthSession, PublicUser \}/);
assert.match(authAPI, /export async function login\(input: LoginInput\): Promise<AuthSession>/);
assert.doesNotMatch(authAPI, /login\(input: LoginInput\): Promise<TokenPair>/);

assert.match(session, /import type \{ AuthSession, PublicUser \}/);
assert.match(session, /publishAuthSession\(session: AuthSession\)/);
assert.doesNotMatch(session, /TokenPair/);

const authSessionBlock = types.match(/export interface AuthSession \{[\s\S]*?\n\}/)?.[0] ?? "";
assert.match(authSessionBlock, /user: PublicUser;/);
assert.doesNotMatch(authSessionBlock, /access_token|refresh_token/);
