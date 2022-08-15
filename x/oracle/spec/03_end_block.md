<!--
order: 3
-->

# End Block

## Tally Exchange Rate Votes

At the end of every block, the `Oracle` module checks whether it's the last block of the `VotePeriod`. If it is, it runs the [Voting Procedure](./01_concepts.md#Voting_Procedure):

1. All current active exchange rates are purged from the store

2. Received votes are organized into ballots by pair. Abstained votes, as well as votes by inactive or jailed validators are ignored

3. Pairs not meeting the following requirements will be dropped:

    - Must appear in the permitted pairs in `Whitelist`
    - Ballot for pair must have at least `VoteThreshold` total vote power

4. For each remaining `pair` with a passing ballot:

    - Tally up votes and find the weighted median exchange rate and winners with `tally()`
    - Iterate through winners of the ballot and add their weight to their running total
    - Set the exchange rate on the blockchain for that pair with `k.SetExchangeRate()`
    - Emit an `exchange_rate_update` event

5. Count up the validators who [missed](./01_concepts.md#Slashing) the Oracle vote and increase the appropriate miss counters

6. If at the end of a `SlashWindow`, penalize validators who have missed more than the penalty threshold (submitted fewer valid votes than `MinValidPerWindow`)

7. Distribute rewards to ballot winners with `k.RewardBallotWinners()`

8. Clear all prevotes (except ones for the next `VotePeriod`) and votes from the store