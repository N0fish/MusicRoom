# PLAYLIST REQUIREMENTS

This document outlines the mandatory requirements for the Playlist, Voting, and Playback logic of the Music Room project, derived strictly from `Agents/subject.txt`.

## 1. Live Music Chain (Dynamic Queue)
The playlist is not a static list; it acts as a dynamic priority queue based on user votes.

> **Subject Quote (IV.2.1):**
> "Live music chain with vote... If a track gets many votes, it **goes up in the list** and is **played earlier**."

### Implementation Requirements:
*   **Dynamic Sorting:** The backend must continuously re-evaluate the order of tracks in the "queue" based on their vote count.
*   **Priority Playback:** Tracks with higher votes must be served before tracks with fewer votes.
*   **Queue Integrity:** Once a track starts playing, it should typically be "locked" or considered "active" (see Section 4), so voting doesn't disrupt the currently playing song.

## 2. Real-Time Collaboration
The playlist editor allows multiple users to modify the state simultaneously, and these changes must be reflected instantly for everyone.

> **Subject Quote (IV.2.3):**
> "Collaborate with your friends... to create playlists in **real-time**. This way, users can create original radio stations."

### Implementation Requirements:
*   **WebSocket Events:** Use `realtime-service` to broadcast events (`track.added`, `track.voted`, `track.moved`) to all connected clients.
*   **Instant Updates:** The mobile app must reflect changes immediately without requiring a manual refresh.

## 3. Conflict Resolution (Competition Management)
The system must handle race conditions where multiple users try to affect the state at the same time.

> **Subject Quote (IV.2.1 & IV.2.3):**
> "You should especially care about the management of **competition problematics**: for instance, if several people vote for different tracks or the same one... or if several people move different tracks..."

> **Subject Quote (IV.3):**
> "The back-end is the reference. It is the keeper and representative of the 'truth'."

### Implementation Requirements:
*   **Atomic Operations:** Voting and track moving operations must be atomic on the backend database level to prevent data corruption.
*   **Single Source of Truth:** The backend decides the final order. Clients simply display what the backend says.
*   **Optimistic UI (Optional but recommended):** Clients can show changes immediately but must rollback if the backend rejects the action (e.g., track was already deleted by someone else).

## 4. Playback State Awareness ("Remote Control")
The detailed requirement implies the system knows *what* is playing to properly schedule "earlier" playback for popular tracks.

> **Subject Quote (IV.5):**
> "Applications must only be **'remote control'** to the back-end"

> **Subject Quote (IV.2.1):**
> "...played **earlier**."

### Implementation Requirements:
*   **Current Track State:** To play something "earlier" (i.e., *next*), the system must define what is "current".
*   **Active Track Logic:** The backend likely needs a field (e.g., `current_track_id` in the Event or Playlist model) to indicate the active track that is effectively "popped" from the voting queue.
*   **Next Track Selection:** Logic must exist to pick the top-voted track as the *next* `current_track_id` when the current one finishes.

## 5. Visibility & Licensing
Access to voting and editing is restricted by visibility and license type.

> **Subject Quote (IV.2.1):**
> "Visibility management (Public/Private)... A license management must be integrated... By default, everyone can vote... With the right license, the invited people are the only one who can vote."

> **Subject Quote (IV.2.3):**
> "With the right license, the invited users are the only ones who can edit the playlist."

### Implementation Requirements:
*   **Permissions Check:** Every API action (vote, add, remove, move) must verify the Event's `Visibility` and `LicenseMode` against the User's status (Owner, Invited, or Public).
