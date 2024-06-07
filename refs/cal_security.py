import numpy as np
import sympy as sym
import scipy as sp
from scipy import stats
from statsmodels.stats.proportion import proportion_confint
import pandas as pd


### helper functions ###

# fast binomial function (scipy gives floats for some values)
def binomial(n, k):
    """
    A fast way to calculate binomial coefficients by Andrew Dalke.
    See http://stackoverflow.com/questions/3025162/statistics-combinations-in-python
    """
    n = int(n)
    k = int(k)
    if 0 <= k <= n:
        ntok = 1
        ktok = 1
        for t in range(1, min(k, n - k) + 1):
            ntok *= n
            ktok *= t
            n -= 1
        return ntok // ktok
    else:
        return 0


# coefficients of generating function corresponding to the sharding problem
def gf_coeffs(n_groups, n_per_group, limit_per_group, total_placed):
    """
    Divide black balls into groups of fixed size, with limits of black balls per group
    n_groups            number of groups
    n_per_group         max balls total per group (of all colors)
    limit_per_group     limit of black balls in a particular group (*strict* inequality)
    total_placed        total number of black balls placed in groups
    """
    
    # get coefficients for a single groups generating function (they are symmetrical)
    # must be passed in descending order
    base_coeffs = []
    for i in range(limit_per_group-1, -1, -1):
        base_coeffs.append(binomial(n_per_group, i))

    # get coefficient for combined generating function: [x^total_placed](g(x))^n_groups
    x = sym.symbols("x")
    gf_base = sym.Poly(base_coeffs, x)
    comb_coeffs = (gf_base**n_groups).coeffs()

    # coefficients are listed in descending order
    # We want coefficient of x^total_placed; list includes x^0
    if total_placed+1 > len(comb_coeffs):
        return 0
    total_placed_coeff = int(comb_coeffs[-(total_placed+1)])

    return total_placed_coeff


# calculate years to failure given probability of failure
def years_to_failure(prob_failure, rounds_per_year):
    # geometric distribution: number of rounds until failure (i.e. a successful attack)
    # https://en.wikipedia.org/wiki/Geometric_distribution
    return (1/prob_failure)/(rounds_per_year)


### Define input parameters ###

# N = 1000        # total number of nodes
# p = 0.15        # actual Byzantine percentage
# K = int(N*p)    # number of Byzantine nodes
# S = 10          # number of shards
# n = N//S        # nodes per shard (assumes S evenly divides N)
# a = 1/3         # Byzantine fault security limit (alpha)
# t = 10000000    # number of trials



## Methodology 2: Analytical calculation ###
SHARD_SIZE = 150

# Correct probability directly from 
def getProbability(S, N, a, K):
    # 正确的计算方法：超几何分布、考虑所有分片
    n_groups = S
    n_per_group = int(N/S)
    limit_per_group = int(np.ceil(a*N/S))
    total_placed = int(K)
    num_success = gf_coeffs(n_groups, n_per_group, limit_per_group, total_placed)
    num_total = binomial(N, K)
    pf_cf = num_success / num_total

    # ytf_hg = years_to_failure(pf_cf, 365)

    return 1-pf_cf

    # 错误的计算方法：二项分布、只考虑一个分片
    # n = int(N/S)
    # bn_single = 1 - sp.stats.binom.cdf(int(np.ceil(a*n-1)), n, K/N)
    # # return bn_single

    # bn_full = 1 - (1 - bn_single)**S
    # return bn_full


# def getSecurity():
#     malicious_rate = [0.25]
#     # shard_num = range(2,60,2)
#     shard_num = [2,4,8,16,64,128,256,1024]
#     security = pd.DataFrame([], index=np.array(malicious_rate))
#     for S in shard_num:
#         security[S] = 0.0
#     # print(security)

#     for k in malicious_rate:
#         n = SHARD_SIZE
#         a = 1/3
#         print('k = ', k)
#         for S in shard_num:
#             prop = getProbability(S, S*n, a, k*S*n)
#             print('S = ', S, 'prop = ', prop)
#             security.loc[k, S] = prop
#     print(security)
#     # security.to_csv('logs/SECURITY-shard_size=%d-tolerance=%.2f.csv'%(SHARD_SIZE, 1/3))

    

# getSecurity()


# 计算不同分片数量S下，多大的分片大小n可以使“没有任何一个分片失效的概率”小于1e-6
def getProWith_S_n():
    a = 0.25 # 固定恶意节点比例为1/5
    shard_num = list(range(2,60,4))+[100,200]
    print(shard_num)
    security = pd.DataFrame([], index=np.array(shard_num))
    shard_size = range(340, 400, 20)
    for n in shard_size:
        security[n] = 0.0
    # print(security)

    last_shardsize = 20
    for S in shard_num:
        r = 1/3
        # print('S = ', S)
        for n in shard_size:
            if n < last_shardsize:
                continue
            prop = getProbability(S, S*n, r, a*S*n)
            print('S = ', S, 'n = ', n,  'prop = ', prop)   
            security.loc[S, n] = prop
            last_shardsize = n
            if prop < 1e-6:
                print('enough shard_size to be safe: ', n)
                break
    print(security)
    security.to_csv('logs/SECURITY-malicious_rate=%.2f-tolerance=%.2f.csv'%(a, 1/3))

    

getProWith_S_n()
# # Print results

# print('\nCorrect: Sampling without replacement (hypergeometric)')
# print('-----------------------------------------------------')
# print('Simulated: {prob}'.format(prob=pf_hg))
# print('Analytical: {prob}'.format(prob=pf_cf))
# print('Analytical (only first shard): {prob}'.format(prob=hg_single))
# print('Years to failure: {ytf}'.format(ytf=ytf_hg))


# print('\n Incorrect: Sampling with replacement (binomial)')
# print('-----------------------------------------------------')
# print('Simulated: {prob}'.format(prob=pf_bn))
# print('Analytical: {prob}'.format(prob=bn_full))
# print('Analytical (only first shard): {prob}'.format(prob=bn_single))
# print('Years to failure: {ytf}'.format(ytf=ytf_bn))





### Comparison: Assuming Ethereum Account Distribution ###

# set up array of K bad nodes and N-K good nodes
def simulateAttack():
    # 参数
    K = 25
    N = 100
    t = 10000
    S = 10
    n = int(N/S)
    a = 1/3

    nodes = np.array([1]*K + [0]*(N-K))
    # print('nodes: ', nodes)
    indices = np.arange(N)

    # Hypergeometric: sampling *without* replacement
    # https://en.wikipedia.org/wiki/Hypergeometric_distribution
    trials_hg = np.full(t, np.nan)

    # Ethereum account data
    # eth = pd.read_excel('./sharding/ethereum_address_balances.xlsx')[['address', 'Eth']]    # top 1000 accounts by balance
    balances = np.full(1000,1).reshape(-1,1)
    eth = pd.DataFrame(balances, columns=['Eth'])
    eth = eth[:N]
    # print('eth: ', eth)

    # run t trials
    for i in range(t):
        if i % 25000 == 0:
            print(i)

        # randomize malicious and 
        # np.random.shuffle(nodes)
        # print('nodes: ', nodes)
        # (eth['Eth']*nodes).sum()/eth['Eth'].sum()   # should be near p
        s_hg_i = np.random.choice(indices, size=[S, n], replace=False)
        # print('s_hg_i: ', s_hg_i)
        s_hg_num = np.reshape((eth['Eth']*nodes).iloc[s_hg_i.flatten()].values, [S,n])
        s_hg_den = np.reshape((eth['Eth']).iloc[s_hg_i.flatten()].values, [S,n])
        s_hg = s_hg_num.sum(axis=1) / s_hg_den.sum(axis=1)


        # trials_bn[i] = ((s_bn >= a).sum() > 0)
        trials_hg[i] = ((s_hg >= a).sum() > 0)

    # failure probabilities
    # pf_bn = trials_bn.sum()/t
    pf_hg = trials_hg.sum()/t

    # Confidence interval (for proportions)
    #https://en.wikipedia.org/wiki/Binomial_proportion_confidence_interval
    # CI = proportion_confint(trials_hg.sum(), t, alpha=0.05, method='normal')  # inaccurate for p near 0 or 1
    CI = proportion_confint(trials_hg.sum(), t, alpha=0.05, method='jeffreys')
    CI_width = (CI[1] - CI[0])/2
    print(pf_hg, CI_width)

# simulateAttack()