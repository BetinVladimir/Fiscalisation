/** @type {import('ts-jest').JestConfigWithTsJest} */
module.exports = {
  preset: "ts-jest",
  testEnvironment: "node",
  testMatch: ["**/*.test.ts", "**/*.test.tsx"],
  transform: {
    "^.+\\.(ts|tsx)$": ["ts-jest", { tsconfig: { jsx: "react" } }],
  },
  modulePathIgnorePatterns: ["\\.store-build"],
  testPathIgnorePatterns: ["/node_modules/", "\\.store-build"],
};
