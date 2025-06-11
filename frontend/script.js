document.addEventListener('DOMContentLoaded', () => {
    const taskForm = document.getElementById('task-form');
    const urlInput = document.getElementById('url-input');
    const taskListDiv = document.getElementById('task-list');
    let isSubmitting = false;

    // 提交新任务的函数
    const submitTask = async () => {
        if (isSubmitting) return;
        isSubmitting = true;

        const url = urlInput.value.trim();
        if (!url) {
            alert('URL 不能为空！');
            isSubmitting = false;
            return;
        }

        // 自动从 URL 末尾提取文件名作为 OBS 对象键的一部分
        const filename = url.substring(url.lastIndexOf('/') + 1);
        const taskData = {
            url: url,
            output_path: `/app/downloads/${filename || 'unknown_file'}`,
            threads: 8 // 可以使用默认值
        };

        try {
            const response = await fetch('/api/download', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(taskData),
            });

            const result = await response.json();
            if (!response.ok) {
                throw new Error(result.error || '提交任务失败');
            }

            urlInput.value = ''; // 成功后清空输入框
            fetchTasks(); // 立即刷新列表以显示新任务
        } catch (error) {
            console.error('提交任务时出错:', error);
            alert(error.message);
        } finally {
            isSubmitting = false;
        }
    };

    taskForm.addEventListener('submit', (e) => {
        e.preventDefault();
        submitTask();
    });

    // 渲染任务列表到页面
    const renderTasks = (tasks) => {
        taskListDiv.innerHTML = '';
        if (!tasks || tasks.length === 0) {
            taskListDiv.innerHTML = '<p>暂无任务，快来提交一个吧！</p>';
            return;
        }

        // 按提交时间倒序排列任务
        tasks.sort((a, b) => new Date(b.submit_time) - new Date(a.submit_time));

        tasks.forEach(task => {
            const taskElement = document.createElement('div');
            taskElement.className = 'task-item';

            const statusClass = `status-${task.status || 'queued'}`;
            const statusText = task.status || '排队中';

            taskElement.innerHTML = `
                <div class="url" title="${task.url}">${task.url}</div>
                <div class="status ${statusClass}">${statusText}</div>
            `;
            taskListDiv.appendChild(taskElement);
        });
    };

    // 从后端获取任务列表数据
    const fetchTasks = async () => {
        try {
            const response = await fetch('/api/tasks');
            if (!response.ok) {
                throw new Error('获取任务列表失败');
            }
            const tasks = await response.json();
            renderTasks(tasks);
        } catch (error) {
            console.error('获取任务时出错:', error);
            taskListDiv.innerHTML = `<p style="color: red;">加载任务列表失败，请检查 API 服务是否正常。</p>`;
        }
    };

    // 页面加载时立即获取一次任务，然后每 5 秒自动刷新一次
    fetchTasks();
    setInterval(fetchTasks, 5000);
});