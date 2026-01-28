package predict

const GraphQLURL = "https://graphql.predict.fun/graphql"
const WsURL = "wss://ws.predict.fun/ws"

const GetMarketsIndexQuery = `query GetMarkets(
  $filter: MarketFilterInput
  $sort: MarketSortInput
  $pagination: ForwardPaginationInput
) {
  markets(filter: $filter, sort: $sort, pagination: $pagination) {
    pageInfo {
      hasNextPage
      startCursor
      endCursor
    }
    edges {
      node {
        id
        status
        isTradingEnabled
        category {
          id
        }
      }
    }
  }
}`

const GetMarketDetailQuery = `query GetMarketFull($marketId: ID!) {
  market(id: $marketId) {
    id
    title
    question
    description
    imageUrl
    createdAt
    status
    isTradingEnabled
    chancePercentage
    spreadThreshold
    shareThreshold
    makerFeeBps
    takerFeeBps
    decimalPrecision
    oracleQuestionId
    oracleTxHash
    conditionId
    resolverAddress
    questionIndex
    category {
      id
      title
      description
      imageUrl
      isNegRisk
      isYieldBearing
      startsAt
      endsAt
      status
      holdersCount
      comments {
        totalCount
      }
      tags {
        edges {
          node {
            id
            name
          }
        }
      }
    }
    statistics {
      totalLiquidityUsd
      volumeTotalUsd
      volume24hUsd
      volume24hChangeUsd
      percentageChanceChange24h
    }
    outcomes {
      edges {
        node {
          id
          index
          name
          status
          onChainId
          bidPriceInCurrency
          askPriceInCurrency
          statistics {
            sharesCount
            positionsValueUsd
          }
          positions {
            totalCount
          }
        }
      }
    }
    resolution {
      id
      name
      index
      status
      createdAt
    }
    statusLogs {
      edges {
        node {
          status
          timestamp
          transactionHash
        }
      }
    }
    bulletinBoardUpdates {
      content
      createdAt
      updatedAt
      transactionHash
    }
  }
}`

const GetMarketHoldersQuery = `query GetMarketHolders(
  $marketId: ID!
  $filter: OutcomeFilterInput
  $pagination: ForwardPaginationInput
) {
  market(id: $marketId) {
    outcomes(filter: $filter) {
      edges {
        node {
          id
          index
          name
          positions(pagination: $pagination) {
            pageInfo {
              hasNextPage
              startCursor
              endCursor
            }
            totalCount
            edges {
              node {
                id
                shares
                valueUsd
                account {
                  address
                  name
                }
              }
              cursor
            }
          }
        }
      }
    }
  }
}`

const GetCommentsQuery = `query GetComments(
  $categoryId: ID!
  $onlyHolders: Boolean
  $pagination: ForwardPaginationInput
  $sortBy: CommentSortBy
  $repliesPagination: ForwardPaginationInput
) {
  comments(
    categoryId: $categoryId
    onlyHolders: $onlyHolders
    pagination: $pagination
    sortBy: $sortBy
  ) {
    pageInfo {
      hasNextPage
      startCursor
      endCursor
    }
    totalCount
    edges {
      ...CommentEdge
    }
  }
}

fragment CommentEdge on CommentEdge {
  cursor
  node {
    ...Comment
    replies(pagination: $repliesPagination) {
      pageInfo {
        hasNextPage
        endCursor
      }
      totalCount
      edges {
        node {
          ...Comment
        }
      }
    }
  }
}

fragment Comment on Comment {
  id
  content
  createdAt
  updatedAt
  likeCount
  isLikedByUser
  replyCount
  reportCount
  account {
    address
    name
    imageUrl
  }
  parentComment {
    id
  }
  replyToComment {
    id
    account {
      name
      address
    }
  }
}`

const GetCategoryTimeseriesQuery = `query GetCategoryTimeseries(
  $categoryId: ID!
  $interval: TimeseriesInterval!
  $pagination: ForwardPaginationInput
) {
  category(id: $categoryId) {
    timeseries(filter: { interval: $interval }, pagination: $pagination) {
      pageInfo {
        hasNextPage
        endCursor
      }
      edges {
        node {
          dataGranularity
          market {
            id
          }
          data {
            edges {
              node {
                x
                y
              }
            }
          }
        }
      }
    }
  }
}`

const LoadMoreRepliesQuery = `query LoadMoreReplies($commentId: ID!, $after: String, $first: Int!) {
  comment(id: $commentId) {
    id
    replyCount
    replies(pagination: { first: $first, after: $after }) {
      pageInfo {
        hasNextPage
        endCursor
      }
      totalCount
      edges {
        node {
          id
          content
          createdAt
          updatedAt
          likeCount
          isLikedByUser
          replyCount
          reportCount
          account {
            address
            name
            imageUrl
          }
          parentComment {
            id
          }
          replyToComment {
            id
            account {
              name
              address
            }
          }
        }
      }
    }
  }
}`
