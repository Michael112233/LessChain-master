from scipy.special import comb

def calculate_probability(c, n=300, b=0.2):
    k = (1/3) * c * n
    malicious_nodes = b * n
    honest_nodes = n - malicious_nodes

    probability = 0
    for i in range(int(k) + 1):
        probability += comb(malicious_nodes, i) * comb(honest_nodes, c * n - i) / comb(n, c * n)
    
    return 1 - probability # 恶意节点超过1/3的概率

# 定义概率阈值
threshold = 4e-2

# 逐渐增加c的值直到找到合适的c
c = 0.01
step = 0.01
while True:
    probability = calculate_probability(c)
    if probability < threshold:
        break
    c += step

print(f"The suitable value for c is approximately {c}, with election fail probability {probability}")