# HNSW Parameter Tuning

### Core Concept: The Trade-off Triangle

Before tuning these parameters, it's crucial to understand that they collectively affect a core **trade-off triangle**:

*   **Recall**: The accuracy of search results. Higher recall means less likely to miss true nearest neighbors.
*   **Query Speed/Latency**: The time required to execute a search. Faster speed means lower latency.
*   **Resource Consumption**: Primarily refers to the memory size occupied by the index and the time required to build the index.

**Typically, you cannot simultaneously achieve the highest recall, fastest query speed, and lowest resource consumption.** Tuning these parameters is about finding the balance point among the three that best suits your application scenario.

---

### Parameter Details

#### 1. `m` (Max Connections)

*   **What it is**: This is the maximum number of connections (out-degree) that each node (vector) in the graph is allowed to have when building the graph. Think of it as a social network where each person can have at most this many "friends".
*   **What it does**:
    *   **Determines graph "density"**: Higher `m` values create denser graphs with more connection options between nodes.
    *   **Affects index quality and memory**: A denser graph typically has higher quality, providing better search paths and thus improving recall potential. However, more connections mean storing more edge information, leading to **larger index files and higher memory usage**.
*   **Performance impact**:
    *   **Increasing `m`** → Improves recall upper limit, but significantly increases memory usage and slightly increases index build time.
    *   **Decreasing `m`** → Reduces memory usage, but may sacrifice recall, especially on complex or high-dimensional datasets.

#### 2. `ef_construction` (EF for Construction)

*   **What it is**: This parameter is used during **index construction**. When a new node is to be inserted into the graph, `ef_construction` defines the size of a dynamic list (candidate pool), from which the algorithm selects the best `m` neighbors to connect to the new node.
*   **What it does**:
    *   **Determines index construction quality**: Higher `ef_construction` means the algorithm "works harder" when finding neighbors for new nodes, searching more widely and finding higher quality connections. A high-quality graph is the foundation of high recall.
*   **Performance impact**:
    *   **Increasing `ef_construction`** → Higher index quality, helping improve final query recall. But **significantly increases index construction time**.
    *   **Decreasing `ef_construction`** → Faster index construction, but may sacrifice index quality, requiring higher `ef_search` during subsequent queries to achieve the same recall.

#### 3. `ef_search` (EF for Search)

*   **What it is**: This parameter is used during **search execution**. Similar to `ef_construction`, it defines the size of a dynamic list (candidate pool) used during search. The algorithm starts from entry points, continuously explores the graph, and places "promising" neighbors into this candidate pool until the search ends.
*   **What it does**:
    *   **Directly controls search depth and breadth**: This is the **"knob" that directly affects recall and speed during queries**.
*   **Performance impact**:
    *   **Increasing `ef_search`** → Broader search scope, visiting more nodes, **significantly improving recall**. However, computation also increases, leading to **increased query latency** (slower speed).
    *   **Decreasing `ef_search`** → Smaller search scope, **faster query speed**, but may miss true nearest neighbors, leading to **decreased recall**.

**Important note**: `ef_search` must be greater than or equal to `k` in the top-k results you want to return.

---

### When to Modify?

Here are some typical application scenarios and corresponding tuning recommendations:

| Scenario | Priority | `m` | `ef_construction` | `ef_search` | Explanation |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Extremely high accuracy requirements**<br>(e.g., academic paper plagiarism detection, image deduplication, legal document comparison) | Recall > Speed > Resources | **Increase** (e.g., 32, 64) | **Increase** (e.g., 400, 500+) | **Significantly increase** (e.g., 100, 200+) | To avoid missing any similar items, sacrifice query speed and resources. First build a high-quality, high-density graph (`m` and `ef_construction`), then perform deep search during queries (`ef_search`). |
| **Extremely high real-time requirements**<br>(e.g., online recommendations, real-time ad serving) | Speed > Recall > Resources | **Keep default or slightly reduce** (e.g., 12, 16) | **Keep default or slightly increase** (e.g., 200, 300) | **Reduce** (e.g., 20, 30) | Prefer to sacrifice some accuracy to ensure low latency for user requests. `ef_search` is the most critical tuning knob and should be minimized while keeping recall within acceptable range. |
| **Very limited memory/storage resources**<br>(e.g., edge devices, mobile deployment) | Resources > Speed/Recall | **Significantly reduce** (e.g., 8, 12) | **Keep default or slightly reduce** (e.g., 100, 200) | **Adjust as needed** | `m` most directly affects memory usage. After reducing `m`, graph quality decreases, may need to appropriately increase `ef_search` to compensate for recall, which will make queries slower. This is a typical space-for-time trade-off. |
| **Frequent data updates, fast index building required**<br>(e.g., news feeds, social information streams) | Index Speed > Query Performance | **Keep default** (e.g., 16) | **Reduce** (e.g., 100) | **Adjust as needed** | `ef_construction` directly affects index build time. Reducing it allows new data to be indexed faster. But index quality decreases, may need higher `ef_search` to compensate for query recall. |
| **General scenario / Balanced requirements** | Balanced | **Default value** (16) | **Default value** (200) | **Default value** (50) | Default values are usually a good starting point that performs reasonably across various datasets. You can make fine adjustments based on your specific business and performance test results. |

### Summary and Tuning Recommendations

1.  **Start with default values**: Don't randomly modify at the beginning. Use default values for baseline testing to understand your system's baseline performance.
2.  **Determine core business metrics**: What can your application least tolerate? High latency, inaccurate results, or memory overflow? This determines your tuning direction.
3.  **Step-by-step adjustment and testing**:
    *   **Step 1: Determine `m`**. `m` primarily affects memory and cannot be changed once the index is built. Based on your memory budget and recall expectations, choose an appropriate `m` (typically 16-48 is a good range).
    *   **Step 2: Determine `ef_construction`**. This value affects index building time. If you're not sensitive to index building time (e.g., one-time construction, infrequent updates), you can set it higher (e.g., 400+) for better index quality.
    *   **Step 3: Dynamically adjust `ef_search`**. This is the most flexible parameter and can be specified for each query. Through continuous testing, find an optimal `ef_search` value that meets recall and latency requirements on your dataset.