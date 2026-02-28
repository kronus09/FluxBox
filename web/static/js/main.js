async function refresh() {
    const res = await fetch('/api/status');
    const data = await res.json();
    document.getElementById('siteCount').innerText = `已聚合 ${data.site_count} 个站点`;
    
    const list = document.getElementById('sourceList');
    list.innerHTML = data.sources.map(s => `
        <tr class="hover:bg-gray-50">
            <td class="py-4">
                <div class="font-medium">${s.name}</div>
                <div class="text-xs text-gray-400 truncate w-48">${s.url}</div>
            </td>
            <td class="py-4">
                <span class="px-2 py-1 text-xs rounded ${s.last_status === 'success' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}">
                    ${s.last_status === 'success' ? '在线' : (s.last_status === 'failed' ? '错误' : '待同步')}
                </span>
                ${s.last_error ? `<p class="text-[10px] text-red-400 mt-1">${s.last_error}</p>` : ''}
            </td>
            <td class="py-4 text-right">
                <button onclick="delSource(${s.id})" class="text-red-400 hover:text-red-600 text-sm">删除</button>
            </td>
        </tr>
    `).join('');
}

async function addSource() {
    const name = document.getElementById('nameInp').value;
    const url = document.getElementById('urlInp').value;
    if (!name || !url) return alert('请填写完整');
    
    await fetch('/api/add', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ name, url })
    });
    document.getElementById('nameInp').value = '';
    document.getElementById('urlInp').value = '';
    refresh();
}

async function delSource(id) {
    if (!confirm('确定删除?')) return;
    await fetch(`/api/delete/${id}`, { method: 'DELETE' });
    refresh();
}

async function aggregate() {
    await fetch('/api/aggregate', { method: 'POST' });
    alert('聚合任务已启动，请稍后刷新页面查看状态');
    setTimeout(refresh, 2000);
}

function copyUrl() {
    const url = window.location.origin + '/config';
    navigator.clipboard.writeText(url);
    alert('已复制聚合配置地址');
}

setInterval(refresh, 5000);
refresh();