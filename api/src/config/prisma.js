const { PrismaClient } = require('@prisma/client');

// Singleton Prisma instance
const prisma = new PrismaClient();

module.exports = prisma;
