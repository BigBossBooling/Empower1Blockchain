# EmPower1 - Architectural Specification: The Wealth Oracle

**Document ID:** EMP-TS-001
**Version:** 1.0
**Status:** Draft
**Author:** Jules, AI Agent (on behalf of The Architect)

---

## 1. Introduction & Purpose

This document provides the technical specification for the **Intelligent Redistribution Engine (IRE)** and its core component, the **Wealth Oracle**. This specification is a direct response to Refinement #1 identified in the Architectural Attestation Document (EMP-AAD-001).

The primary function of the IRE is to execute the core mission of the EmPower1 Blockchain: to bridge the wealth gap by algorithmically redistributing resources. The Wealth Oracle is the critical component that provides the necessary data to make this redistribution fair, transparent, and secure.

This document aims to resolve the foundational architectural challenge of enabling wealth analysis while respecting user privacy and maintaining decentralization.

---

## 2. The Architectural Paradox: Privacy vs. Analysis

The initial architectural concept presented a fundamental conflict:

*   **The Goal of Analysis:** The system must analyze user data to gauge wealth/need to execute its redistribution mandate.
*   **The Goal of Privacy:** The system aims to provide users with strong privacy guarantees, mentioning advanced cryptography like zk-SNARKs.

These two goals are mutually exclusive in a simple architecture. One cannot analyze data that is cryptographically hidden. A naive implementation would force a choice between mission effectiveness and user privacy, or would require a powerful, centralized entity to have access to all user data, undermining decentralization.

Our proposed architecture resolves this paradox by shifting from a mandatory, opaque system to a voluntary, transparent, and user-centric model.

---

## 3. Proposed Architecture: A Hybrid Opt-In Model

The core principle of the refined architecture is **user sovereignty**. Participation in the Intelligent Redistribution Engine is **strictly voluntary (opt-in)**. By default, all user accounts and transactions are private and are not subject to wealth analysis or the 9% transaction tax.

### 3.1. The Opt-In Mechanism

*   Users who wish to participate in the ecosystem—either as potential recipients of stimulus or as contributors—must actively opt-in through a special transaction.
*   This transaction links their primary address to a **Decentralized Identity (DID)**. This DID serves as the anchor for their "Wealth/Needs Score."
*   This act of opting in signifies explicit user consent for the Wealth Oracle to analyze the data associated with their DID and linked address.

### 3.2. The Wealth/Needs Score

The system calculates a **Wealth/Needs Score (WNS)** for each opted-in user. This score is a single, normalized value that the on-chain smart contracts use to determine tax rates and stimulus eligibility. The WNS is calculated using a hybrid data model:

*   **Tier 1: On-Chain Data (Mandatory for Opt-In):**
    *   The Wealth Oracle analyzes publicly available data for the user's linked address.
    *   Metrics include: Native token balance, staking amounts and duration, transaction volume and frequency, and holdings of other recognized tokens on the EmPower1 chain.
    *   This provides a baseline score that is transparent and verifiable by anyone.

*   **Tier 2: Off-Chain Verifiable Credentials (Optional & User-Provided):**
    *   To achieve a more accurate WNS, users can voluntarily and selectively attach **Verifiable Credentials (VCs)** to their DID.
    *   Examples: A VC from a trusted third party verifying residence in a developing nation, a VC confirming status as a student, or a VC from a DeFi protocol on another chain proving a low net worth.
    *   This allows users to provide a more holistic picture of their situation, enabling the system to be more equitable. For example, a user with low on-chain assets but high off-chain wealth would not be able to unfairly claim stimulus.

### 3.3. The Wealth Oracle Network

The "Wealth Oracle" is not a single server but a **decentralized network of oracle nodes**.

*   **Function:** The oracle nodes are responsible for:
    1.  Reading the on-chain data for all opted-in addresses.
    2.  Securely receiving and verifying the cryptographic proofs of any user-submitted VCs.
    3.  Executing a publicly auditable algorithm to compute the WNS for each user.
    4.  Submitting the WNS to the EmPower1 blockchain, where it is stored in a dedicated on-chain registry.
*   **Decentralization & Security:** The oracle network must reach an internal consensus on the scores before submitting them. The use of multiple nodes prevents any single node from corrupting the data. The nodes would be required to be staked and could be slashed for malicious behavior.

---

## 4. Governance of the Oracle

To ensure long-term integrity and alignment with the project's mission, the Wealth Oracle Network and its parameters will be governed by the **EmPower1 DAO**. The DAO will have authority over:

*   The specific algorithm used to calculate the WNS.
*   The list of trusted issuers for Verifiable Credentials.
*   The requirements for becoming an oracle node operator.
*   The fee and reward structure for oracle nodes.

---

## 5. Security, Integrity & Mitigation of Gaming

*   **Sybil Resistance:** Requiring a DID and a potential fee for the opt-in process significantly increases the cost and complexity of a Sybil attack (where a single user creates many addresses to unfairly receive stimulus).
*   **Data Privacy:** Since off-chain data is only provided voluntarily as VCs, the user remains in full control of their private information. The VCs themselves are designed to reveal the minimum necessary information (e.g., "Is user a resident of country X?" -> Yes/No, without revealing the user's name or address).
*   **Transparency:** The WNS algorithm will be open source and auditable by the community, ensuring transparency in how scores are calculated.

---

## 6. Conclusion

This specification proposes a hybrid, opt-in architecture for the Intelligent Redistribution Engine. This model resolves the privacy-versus-analysis paradox by making privacy the default and participation a sovereign choice.

By leveraging DIDs, Verifiable Credentials, and a decentralized oracle network under DAO governance, this architecture provides a clear and viable path to implementing the core mission of the EmPower1 Blockchain in a manner that is secure, transparent, and respectful of user autonomy. This design is the foundational first step in turning the project's visionary goals into an engineering reality.
