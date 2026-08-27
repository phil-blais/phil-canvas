// Display identity for presence: what a room member is called.

import type { User } from 'firebase/auth'

const ADJECTIVES = ['Swift', 'Quiet', 'Bright', 'Brave', 'Jolly', 'Nimble', 'Sunny', 'Clever']
const ANIMALS = ['Otter', 'Falcon', 'Heron', 'Lynx', 'Marlin', 'Badger', 'Sparrow', 'Bison']

export function randomGuestName(): string {
  const adj = ADJECTIVES[Math.floor(Math.random() * ADJECTIVES.length)]
  const animal = ANIMALS[Math.floor(Math.random() * ANIMALS.length)]
  return `${adj} ${animal}`
}

/** Display name for a room member: the admin's Firebase name/email when hosting, else a random guest name. */
export function resolveUsername(role: 'admin' | 'guest', adminUser: User | undefined): string {
  if (role !== 'admin') return randomGuestName()
  return adminUser?.displayName ?? adminUser?.email ?? 'Host'
}
