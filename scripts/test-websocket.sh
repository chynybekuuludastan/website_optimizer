#!/bin/bash

# Create static directory if it doesn't exist
mkdir -p static

# Save the HTML file
cat > static/websocket-test.html << 'EOL'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Website Analyzer - WebSocket Tester</title>
    
    <!-- CSS -->
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
    <style>
        :root {
            --primary-color: #3498db;
            --secondary-color: #2ecc71;
            --danger-color: #e74c3c;
            --warning-color: #f39c12;
            --dark-color: #2c3e50;
            --light-color: #ecf0f1;
        }
        
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background-color: #f9f9f9;
            padding: 20px;
        }
        
        .card {
            border-radius: 8px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
            margin-bottom: 20px;
        }
        
        .card-header {
            background-color: var(--dark-color);
            color: white;
            font-weight: bold;
            border-top-left-radius: 8px !important;
            border-top-right-radius: 8px !important;
        }
        
        .connection-status {
            padding: 6px 12px;
            border-radius: 20px;
            font-size: 14px;
            font-weight: bold;
        }
        
        .status-connected {
            background-color: var(--secondary-color);
            color: white;
        }
        
        .status-disconnected {
            background-color: var(--danger-color);
            color: white;
        }
        
        .status-connecting {
            background-color: var(--warning-color);
            color: white;
        }
        
        .progress {
            height: 20px;
            border-radius: 4px;
        }
        
        .analyzer-section {
            border-left: 4px solid var(--primary-color);
            padding-left: 15px;
            margin-bottom: 15px;
        }
        
        .analyzer-title {
            font-weight: bold;
            color: var(--dark-color);
        }
        
        .analyzer-content {
            padding: 10px;
            background-color: var(--light-color);
            border-radius: 4px;
            max-height: 200px;
            overflow-y: auto;
        }
        
        #messageLog {
            height: 300px;
            overflow-y: auto;
            font-family: monospace;
            font-size: 12px;
            background-color: #2c3e50;
            color: #ecf0f1;
            padding: 10px;
            border-radius: 4px;
        }
        
        .message-in {
            color: #2ecc71;
        }
        
        .message-out {
            color: #3498db;
        }
        
        .message-error {
            color: #e74c3c;
        }
        
        .chart-container {
            position: relative;
            height: 200px;
            width: 100%;
        }
        
        .control-button {
            margin-right: 8px;
            margin-bottom: 8px;
        }
        
        .heartbeat-indicator {
            display: inline-block;
            width: 12px;
            height: 12px;
            border-radius: 50%;
            background-color: var(--secondary-color);
            margin-right: 5px;
            animation: pulse 1.5s infinite;
        }
        
        @keyframes pulse {
            0% { opacity: 1; }
            50% { opacity: 0.3; }
            100% { opacity: 1; }
        }
        
        .debug-info {
            font-size: 12px;
            font-family: monospace;
        }
        
        .latency-badge {
            font-size: 12px;
            background-color: var(--dark-color);
            color: white;
            padding: 3px 6px;
            border-radius: 4px;
        }
        
        /* Media Queries for Responsive Design */
        @media (max-width: 768px) {
            .container {
                padding: 0;
            }
            
            .card {
                margin-bottom: 15px;
            }
            
            .control-button {
                margin-bottom: 10px;
                width: 100%;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1 class="text-center mb-4">Website Analyzer WebSocket Tester</h1>
        
        <!-- Connection Status Card -->
        <div class="card mb-4">
            <div class="card-header d-flex justify-content-between align-items-center">
                <span>WebSocket Connection</span>
                <span id="connectionStatus" class="connection-status status-disconnected">
                    Disconnected
                </span>
            </div>
            <div class="card-body">
                <div class="row">
                    <div class="col-md-8">
                        <div class="input-group mb-3">
                            <input type="text" id="websiteUrl" class="form-control" placeholder="Enter website URL (e.g., https://example.com)" aria-label="Website URL">
                            <button 
                                class="btn btn-primary" 
                                type="button" 
                                id="analyzeBtn"
                                hx-post="/api/analysis"
                                hx-trigger="click"
                                hx-vals='js:{url: document.getElementById("websiteUrl").value}'
                                hx-target="#analysisIdContainer"
                                hx-swap="innerHTML"
                                hx-indicator="#indicator">
                                Analyze
                            </button>
                        </div>
                        <div id="analysisIdContainer" class="mb-3"></div>
                    </div>
                    <div class="col-md-4">
                        <div class="d-flex justify-content-end align-items-center">
                            <div class="me-3">
                                <span id="messageCounter" class="badge bg-secondary">0 messages</span>
                            </div>
                            <div class="me-3">
                                <span id="latencyIndicator" class="latency-badge">- ms</span>
                            </div>
                            <button id="clearBtn" class="btn btn-outline-secondary btn-sm">Clear</button>
                        </div>
                    </div>
                </div>
                
                <div class="row">
                    <div class="col-12">
                        <div class="progress mb-3">
                            <div id="analysisProgress" class="progress-bar progress-bar-striped progress-bar-animated" role="progressbar" style="width: 0%" aria-valuenow="0" aria-valuemin="0" aria-valuemax="100">0%</div>
                        </div>
                    </div>
                </div>
                
                <div class="row">
                    <div class="col-12">
                        <div class="btn-group" role="group" aria-label="Control Actions">
                            <button 
                                id="pauseBtn" 
                                class="btn btn-warning control-button"
                                hx-ws="send"
                                disabled>
                                <i class="fas fa-pause"></i> Pause
                            </button>
                            <button 
                                id="resumeBtn" 
                                class="btn btn-success control-button"
                                hx-ws="send"
                                disabled>
                                <i class="fas fa-play"></i> Resume
                            </button>
                            <button 
                                id="cancelBtn" 
                                class="btn btn-danger control-button"
                                hx-ws="send"
                                disabled>
                                <i class="fas fa-stop"></i> Cancel
                            </button>
                            <button 
                                id="updateParamsBtn" 
                                class="btn btn-info control-button"
                                hx-ws="send"
                                disabled>
                                <i class="fas fa-cog"></i> Update Params
                            </button>
                        </div>
                    </div>
                </div>
                
                <div class="row mt-3">
                    <div class="col-md-6">
                        <div class="form-group">
                            <label for="paramName">Parameter Name:</label>
                            <input type="text" id="paramName" class="form-control" placeholder="e.g., check_mobile">
                        </div>
                    </div>
                    <div class="col-md-6">
                        <div class="form-group">
                            <label for="paramValue">Parameter Value:</label>
                            <input type="text" id="paramValue" class="form-control" placeholder="e.g., true">
                        </div>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- Analysis Status Card -->
        <div class="row">
            <div class="col-md-6">
                <div class="card">
                    <div class="card-header">
                        Analysis Status
                    </div>
                    <div class="card-body">
                        <div id="analysisStatus" class="mb-3">
                            <p class="text-muted">No active analysis</p>
                        </div>
                        
                        <h5>Analysis Categories:</h5>
                        <div id="analyzerCategories">
                            <div class="analyzer-section" id="seo-section">
                                <div class="analyzer-title">SEO <span class="badge bg-secondary" id="seo-badge">Pending</span></div>
                                <div class="analyzer-content" id="seo-content">
                                    <p class="text-muted">Waiting for results...</p>
                                </div>
                            </div>
                            
                            <div class="analyzer-section" id="performance-section">
                                <div class="analyzer-title">Performance <span class="badge bg-secondary" id="performance-badge">Pending</span></div>
                                <div class="analyzer-content" id="performance-content">
                                    <p class="text-muted">Waiting for results...</p>
                                </div>
                            </div>
                            
                            <div class="analyzer-section" id="accessibility-section">
                                <div class="analyzer-title">Accessibility <span class="badge bg-secondary" id="accessibility-badge">Pending</span></div>
                                <div class="analyzer-content" id="accessibility-content">
                                    <p class="text-muted">Waiting for results...</p>
                                </div>
                            </div>
                            
                            <div class="analyzer-section" id="content-section">
                                <div class="analyzer-title">Content <span class="badge bg-secondary" id="content-badge">Pending</span></div>
                                <div class="analyzer-content" id="content-content">
                                    <p class="text-muted">Waiting for results...</p>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            
            <div class="col-md-6">
                <div class="card">
                    <div class="card-header">
                        Message Log
                    </div>
                    <div class="card-body">
                        <div id="messageLog" class="mb-3"></div>
                        
                        <div class="d-flex justify-content-between align-items-center">
                            <div>
                                <span id="ws-status" class="debug-info">WebSocket: Disconnected</span>
                            </div>
                            <div>
                                <span id="reconnect-attempts" class="debug-info">Reconnects: 0</span>
                            </div>
                            <div>
                                <span id="last-ping" class="debug-info">Last ping: -</span>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- Results & Visualization Card -->
        <div class="card mt-4">
            <div class="card-header">
                Analysis Results & Visualization
            </div>
            <div class="card-body">
                <div class="row">
                    <div class="col-md-6">
                        <h5>Category Scores</h5>
                        <div class="chart-container">
                            <canvas id="scoreChart"></canvas>
                        </div>
                    </div>
                    <div class="col-md-6">
                        <h5>Recommendations</h5>
                        <div id="recommendationsContainer" class="p-3 bg-light rounded">
                            <p class="text-muted">No recommendations available yet.</p>
                        </div>
                    </div>
                </div>
                
                <div class="row mt-4">
                    <div class="col-12">
                        <h5>Analysis Timeline</h5>
                        <div class="timeline-container p-3 bg-light rounded">
                            <div id="timelineContainer" style="height: 100px;">
                                <p class="text-muted">No timeline data available yet.</p>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
    
    <!-- Loading indicator -->
    <div id="indicator" class="htmx-indicator fixed-top w-100 bg-primary text-white text-center py-2" style="display:none;">
        Processing request...
    </div>
    
    <!-- Scripts -->
    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/Chart.js/3.9.1/chart.min.js"></script>
    <script src="https://unpkg.com/htmx.org@1.9.6"></script>
    <script src="https://unpkg.com/htmx.org/dist/ext/ws.js"></script>
    <script>
        // Initialize HTMX WebSocket Extension
        htmx.defineExtension('ws-ext', {
            onEvent: function(name, evt) {
                if (name === 'htmx:wsOpen') {
                    console.log('WebSocket opened');
                    updateConnectionStatus('connected');
                }
                if (name === 'htmx:wsClose') {
                    console.log('WebSocket closed');
                    updateConnectionStatus('disconnected');
                }
                if (name === 'htmx:wsError') {
                    console.error('WebSocket error', evt.detail.error);
                    updateConnectionStatus('disconnected');
                    logMessage('WebSocket error: ' + evt.detail.error, 'error');
                }
                if (name === 'htmx:wsBeforeMessage') {
                    const messageData = JSON.parse(evt.detail.message);
                    updateLatencyIndicator();
                    incrementMessageCounter();
                    logMessage('⬇️ RECEIVED: ' + evt.detail.message, 'in');
                    
                    // Process message based on type
                    processWebSocketMessage(messageData);
                }
                if (name === 'htmx:wsSend') {
                    logMessage('⬆️ SENT: ' + evt.detail.message, 'out');
                }
            }
        });
        
        // Initialize variables
        let socket = null;
        let analysisID = null;
        let messageCount = 0;
        let reconnectCount = 0;
        let lastMessageTime = null;
        let analysisEvents = [];
        let scoreChart = null;
        let currentAnalysisData = {
            start_time: null,
            category_scores: {},
            recommendations: [],
            issues: []
        };
        
        // Initialize the page
        document.addEventListener('DOMContentLoaded', function() {
            // Register the extension
            htmx.extendNative('ws-ext');
            
            // Initialize Charts
            initScoreChart();
            
            // Set up event listeners
            document.getElementById('clearBtn').addEventListener('click', clearLogs);
            document.getElementById('pauseBtn').addEventListener('click', sendPauseCommand);
            document.getElementById('resumeBtn').addEventListener('click', sendResumeCommand);
            document.getElementById('cancelBtn').addEventListener('click', sendCancelCommand);
            document.getElementById('updateParamsBtn').addEventListener('click', sendUpdateParamsCommand);
            
            // Set up event listeners for HTMX responses
            document.body.addEventListener('htmx:afterRequest', function(evt) {
                if (evt.detail.path === '/api/analysis' && evt.detail.xhr.status === 201) {
                    try {
                        const response = JSON.parse(evt.detail.xhr.response);
                        if (response.success && response.data && response.data.analysis_id) {
                            analysisID = response.data.analysis_id;
                            connectToWebSocket(analysisID);
                            resetAnalysisState();
                            enableControlButtons();
                        }
                    } catch (e) {
                        console.error('Error parsing analysis response', e);
                    }
                }
            });
        });
        
        // Connect to WebSocket for a specific analysis
        function connectToWebSocket(id) {
            if (socket) {
                socket.close();
            }
            
            updateConnectionStatus('connecting');
            
            const wsUrl = `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/ws/analysis/${id}`;
            socket = new WebSocket(wsUrl);
            
            socket.onopen = function() {
                updateConnectionStatus('connected');
                document.getElementById('ws-status').textContent = `WebSocket: Connected to ${id}`;
                logMessage(`Connected to WebSocket for analysis ID: ${id}`, 'info');
            };
            
            socket.onclose = function() {
                updateConnectionStatus('disconnected');
                document.getElementById('ws-status').textContent = 'WebSocket: Disconnected';
                disableControlButtons();
                
                // Attempt to reconnect if we have an analysis ID
                if (analysisID) {
                    setTimeout(function() {
                        reconnectCount++;
                        document.getElementById('reconnect-attempts').textContent = `Reconnects: ${reconnectCount}`;
                        connectToWebSocket(analysisID);
                    }, 3000);
                }
            };
            
            socket.onerror = function(error) {
                logMessage(`WebSocket error: ${error}`, 'error');
            };
            
            socket.onmessage = function(event) {
                const data = JSON.parse(event.data);
                lastMessageTime = new Date();
                messageCount++;
                document.getElementById('messageCounter').textContent = `${messageCount} messages`;
                
                logMessage(`Received: ${event.data}`, 'in');
                processWebSocketMessage(data);
            };
            
            // Set up the HTMX WebSocket connection
            htmx.createWebSocket(wsUrl);
        }
        
        // Process WebSocket messages
        function processWebSocketMessage(data) {
            if (!data) return;
            
            // Record the event for timeline
            recordAnalysisEvent(data);
            
            // Check if this is a type property or a success property response
            if (data.type) {
                // This is a message with a type field (from websocket hub)
                handleTypedMessage(data);
            } else if (data.success !== undefined) {
                // This is a standard API response
                handleApiResponse(data);
            }
        }
        
        // Handle typed messages (from websocket hub)
        function handleTypedMessage(data) {
            const timestamp = new Date();
            
            switch(data.type) {
                case 'analysis_started':
                    updateAnalysisStatus('Analysis started', 'info');
                    updateProgressBar(0);
                    currentAnalysisData.start_time = timestamp;
                    break;
                    
                case 'analysis_progress':
                    updateProgressBar(data.progress || 0);
                    updateAnalysisStatus(`Analysis in progress: ${data.status}`, 'info');
                    
                    // Update specific analyzer section if category is provided
                    if (data.category && data.category !== 'all') {
                        updateAnalyzerSection(data.category, data.status, data.data);
                    }
                    break;
                    
                case 'partial_results':
                    if (data.category) {
                        updateAnalyzerSection(data.category, 'partial results', data.data);
                        
                        // If score is available, update the chart
                        if (data.data && data.data.results && data.data.results.score) {
                            currentAnalysisData.category_scores[data.category] = data.data.results.score;
                            updateScoreChart();
                        }
                    }
                    break;
                    
                case 'analysis_completed':
                    updateAnalysisStatus('Analysis completed successfully!', 'success');
                    updateProgressBar(100);
                    disableControlButtons();
                    
                    // Update overall score if available
                    if (data.data && data.data.overall_score) {
                        document.getElementById('analysisStatus').innerHTML = 
                            `<div class="alert alert-success">Analysis completed with overall score: ${data.data.overall_score.toFixed(1)}</div>`;
                    }
                    
                    // Update category scores if available
                    if (data.data && data.data.category_scores) {
                        currentAnalysisData.category_scores = data.data.category_scores;
                        updateScoreChart();
                    }
                    break;
                    
                case 'analysis_error':
                    updateAnalysisStatus(`Error: ${data.data.error || 'Unknown error'}`, 'danger');
                    disableControlButtons();
                    break;
                    
                case 'control_response':
                    updateAnalysisStatus(`Control action ${data.data.action} ${data.data.result.success ? 'succeeded' : 'failed'}`, 
                        data.data.result.success ? 'success' : 'danger');
                    break;
                    
                case 'ping':
                    document.getElementById('last-ping').textContent = `Last ping: ${new Date().toTimeString().split(' ')[0]}`;
                    // Automatically respond with pong
                    sendPongMessage();
                    break;
            }
        }
        
        // Handle standard API responses
        function handleApiResponse(data) {
            if (data.success) {
                if (data.data) {
                    // Check if this is analysis data
                    if (data.data.id && data.data.status) {
                        updateAnalysisStatus(`Analysis status: ${data.data.status}`, 'info');
                        
                        if (data.data.status === 'completed' || data.data.status === 'failed') {
                            disableControlButtons();
                            
                            if (data.data.status === 'completed' && data.data.overall_score) {
                                document.getElementById('analysisStatus').innerHTML = 
                                    `<div class="alert alert-success">Analysis completed with overall score: ${data.data.overall_score.toFixed(1)}</div>`;
                                
                                // Update score chart if category scores are available
                                if (data.data.category_scores) {
                                    currentAnalysisData.category_scores = data.data.category_scores;
                                    updateScoreChart();
                                }
                            }
                            
                            if (data.data.status === 'failed') {
                                document.getElementById('analysisStatus').innerHTML = 
                                    `<div class="alert alert-danger">Analysis failed</div>`;
                            }
                        }
                    }
                }
            } else {
                // Handle error
                updateAnalysisStatus(`Error: ${data.error || 'Unknown error'}`, 'danger');
            }
        }
        
        // Update connection status UI
        function updateConnectionStatus(status) {
            const statusElement = document.getElementById('connectionStatus');
            statusElement.className = 'connection-status';
            
            switch(status) {
                case 'connected':
                    statusElement.classList.add('status-connected');
                    statusElement.innerHTML = '<i class="fas fa-plug"></i> Connected';
                    break;
                case 'disconnected':
                    statusElement.classList.add('status-disconnected');
                    statusElement.innerHTML = '<i class="fas fa-unlink"></i> Disconnected';
                    break;
                case 'connecting':
                    statusElement.classList.add('status-connecting');
                    statusElement.innerHTML = '<i class="fas fa-sync fa-spin"></i> Connecting...';
                    break;
            }
        }
        
        // Update analysis status UI
        function updateAnalysisStatus(message, type) {
            const statusElement = document.getElementById('analysisStatus');
            let alertClass = 'alert-info';
            
            switch(type) {
                case 'success': alertClass = 'alert-success'; break;
                case 'danger': alertClass = 'alert-danger'; break;
                case 'warning': alertClass = 'alert-warning'; break;
            }
            
            statusElement.innerHTML = `<div class="alert ${alertClass}">${message}</div>`;
        }
        
        // Update progress bar
        function updateProgressBar(progress) {
            const progressBar = document.getElementById('analysisProgress');
            progressBar.style.width = `${progress}%`;
            progressBar.setAttribute('aria-valuenow', progress);
            progressBar.textContent = `${Math.round(progress)}%`;
            
            // Change color based on progress
            progressBar.className = 'progress-bar progress-bar-striped progress-bar-animated';
            if (progress < 30) {
                progressBar.classList.add('bg-danger');
            } else if (progress < 70) {
                progressBar.classList.add('bg-warning');
            } else {
                progressBar.classList.add('bg-success');
            }
        }
        
        // Update analyzer section
        function updateAnalyzerSection(category, status, data) {
            const sectionElement = document.getElementById(`${category}-section`);
            const badgeElement = document.getElementById(`${category}-badge`);
            const contentElement = document.getElementById(`${category}-content`);
            
            if (!sectionElement || !badgeElement || !contentElement) return;
            
            // Update badge
            badgeElement.className = 'badge';
            switch(status) {
                case 'running':
                case 'analyzing':
                    badgeElement.classList.add('bg-primary');
                    badgeElement.textContent = 'In Progress';
                    break;
                case 'completed':
                    badgeElement.classList.add('bg-success');
                    badgeElement.textContent = 'Completed';
                    break;
                case 'failed':
                    badgeElement.classList.add('bg-danger');
                    badgeElement.textContent = 'Failed';
                    break;
                case 'partial results':
                    badgeElement.classList.add('bg-info');
                    badgeElement.textContent = 'Partial Results';
                    break;
                default:
                    badgeElement.classList.add('bg-secondary');
                    badgeElement.textContent = status || 'Pending';
            }
            
            // Update content based on data
            if (data) {
                let contentHtml = '';
                
                // Check if we have analyzer results
                if (data.analyzer) {
                    contentHtml += `<p><strong>Analyzer:</strong> ${data.analyzer}</p>`;
                }
                
                // Check if we have a message
                if (data.message) {
                    contentHtml += `<p><em>${data.message}</em></p>`;
                }
                
                // Check if we have progress
                if (data.overall_progress) {
                    contentHtml += `<div class="progress mb-2" style="height: 10px;">
                        <div class="progress-bar bg-info" role="progressbar" style="width: ${data.overall_progress}%" 
                        aria-valuenow="${data.overall_progress}" aria-valuemin="0" aria-valuemax="100"></div>
                    </div>`;
                }
                
                // Check if we have results data
                if (data.results) {
                    contentHtml += `<div class="mt-2">`;
                    
                    // For score results
                    if (data.results.score !== undefined) {
                        const scorePercent = parseFloat(data.results.score).toFixed(1);
                        let scoreClass = 'text-danger';
                        if (scorePercent >= 70) scoreClass = 'text-success';
                        else if (scorePercent >= 50) scoreClass = 'text-warning';
                        
                        contentHtml += `<p><strong>Score: </strong><span class="${scoreClass}">${scorePercent}</span></p>`;
                    }
                    
                    // For issues
                    if (data.results.issues && data.results.issues.length) {
                        contentHtml += `<p><strong>Issues:</strong></p>
                        <ul class="small">`;
                        
                        data.results.issues.slice(0, 3).forEach(issue => {
                            contentHtml += `<li>${issue.description || issue.type} (${issue.severity})</li>`;
                        });
                        
                        if (data.results.issues.length > 3) {
                            contentHtml += `<li>...and ${data.results.issues.length - 3} more issues</li>`;
                        }
                        
                        contentHtml += `</ul>`;
                    }
                    
                    contentHtml += `</div>`;
                }
                
                if (contentHtml) {
                    contentElement.innerHTML = contentHtml;
                }
            }
        }
        
        // Initialize score chart
        function initScoreChart() {
            const ctx = document.getElementById('scoreChart').getContext('2d');
            
            scoreChart = new Chart(ctx, {
                type: 'radar',
                data: {
                    labels: ['SEO', 'Performance', 'Accessibility', 'Content', 'Mobile', 'Security'],
                    datasets: [{
                        label: 'Category Scores',
                        data: [0, 0, 0, 0, 0, 0],
                        backgroundColor: 'rgba(52, 152, 219, 0.2)',
                        borderColor: 'rgba(52, 152, 219, 1)',
                        pointBackgroundColor: 'rgba(52, 152, 219, 1)',
                        pointBorderColor: '#fff',
                        pointHoverBackgroundColor: '#fff',
                        pointHoverBorderColor: 'rgba(52, 152, 219, 1)'
                    }]
                },
                options: {
                    scales: {
                        r: {
                            beginAtZero: true,
                            max: 100,
                            ticks: {
                                stepSize: 20
                            }
                        }
                    },
                    plugins: {
                        legend: {
                            display: false
                        }
                    }
                }
            });
        }
        
        // Update score chart with current data
        function updateScoreChart() {
            if (!scoreChart) return;
            
            const categories = ['seo', 'performance', 'accessibility', 'content', 'mobile', 'security'];
            const scores = categories.map(cat => {
                return currentAnalysisData.category_scores[cat] || 0;
            });
            
            scoreChart.data.datasets[0].data = scores;
            scoreChart.update();
            
            // Also update the recommendations section if we have recommendations
            updateRecommendationsDisplay();
        }
        
        // Update recommendations display
        function updateRecommendationsDisplay() {
            const container = document.getElementById('recommendationsContainer');
            
            // Check if we have recommendations
            if (currentAnalysisData.recommendations && currentAnalysisData.recommendations.length > 0) {
                let html = '<ul class="list-group">';
                
                currentAnalysisData.recommendations.forEach(rec => {
                    let priorityClass = 'list-group-item-info';
                    if (rec.priority === 'high') priorityClass = 'list-group-item-danger';
                    else if (rec.priority === 'medium') priorityClass = 'list-group-item-warning';
                    
                    html += `<li class="list-group-item ${priorityClass}">
                        <div class="d-flex justify-content-between">
                            <span>${rec.description || rec.title}</span>
                            <span class="badge bg-secondary">${rec.category}</span>
                        </div>
                    </li>`;
                });
                
                html += '</ul>';
                container.innerHTML = html;
            } else {
                container.innerHTML = '<p class="text-muted">No recommendations available yet.</p>';
            }
        }
        
        // Reset analysis state
        function resetAnalysisState() {
            // Reset progress bar
            updateProgressBar(0);
            
            // Reset analyzer sections
            const categories = ['seo', 'performance', 'accessibility', 'content'];
            categories.forEach(cat => {
                const badgeElement = document.getElementById(`${cat}-badge`);
                const contentElement = document.getElementById(`${cat}-content`);
                
                if (badgeElement) {
                    badgeElement.className = 'badge bg-secondary';
                    badgeElement.textContent = 'Pending';
                }
                
                if (contentElement) {
                    contentElement.innerHTML = '<p class="text-muted">Waiting for results...</p>';
                }
            });
            
            // Reset analysis status
            document.getElementById('analysisStatus').innerHTML = '<p class="text-muted">Starting analysis...</p>';
            
            // Reset current analysis data
            currentAnalysisData = {
                start_time: new Date(),
                category_scores: {},
                recommendations: [],
                issues: []
            };
            
            // Reset charts
            if (scoreChart) {
                scoreChart.data.datasets[0].data = [0, 0, 0, 0, 0, 0];
                scoreChart.update();
            }
            
            // Reset timeline
            analysisEvents = [];
            document.getElementById('timelineContainer').innerHTML = '<p class="text-muted">Collecting timeline data...</p>';
            
            // Reset recommendations
            document.getElementById('recommendationsContainer').innerHTML = '<p class="text-muted">No recommendations available yet.</p>';
        }
        
        // Record analysis event for timeline
        function recordAnalysisEvent(data) {
            const event = {
                timestamp: new Date(),
                data: data
            };
            
            analysisEvents.push(event);
            
            // Update timeline if we have enough events
            if (analysisEvents.length > 1) {
                updateTimeline();
            }
        }
        
        // Update timeline visualization
        function updateTimeline() {
            const container = document.getElementById('timelineContainer');
            const startTime = analysisEvents[0].timestamp;
            const currentTime = new Date();
            const totalDuration = currentTime - startTime;
            
            if (totalDuration <= 0) return;
            
            let html = '<div class="timeline-line" style="position:relative; height:30px; background-color:#f5f5f5; border-radius:4px; margin-bottom:20px;">';
            
            // Add event markers
            analysisEvents.forEach(event => {
                const eventTime = event.timestamp;
                const position = ((eventTime - startTime) / totalDuration) * 100;
                
                let color = '#3498db'; // Default blue
                let title = 'Event';
                
                // Determine color and title based on event type
                if (event.data.type) {
                    switch(event.data.type) {
                        case 'analysis_started':
                            color = '#2ecc71'; // Green
                            title = 'Started';
                            break;
                        case 'analysis_progress':
                            color = '#3498db'; // Blue
                            title = `Progress: ${Math.round(event.data.progress || 0)}%`;
                            break;
                        case 'partial_results':
                            color = '#9b59b6'; // Purple
                            title = `Partial: ${event.data.category || 'unknown'}`;
                            break;
                        case 'analysis_completed':
                            color = '#2ecc71'; // Green
                            title = 'Completed';
                            break;
                        case 'analysis_error':
                            color = '#e74c3c'; // Red
                            title = 'Error';
                            break;
                    }
                }
                
                html += `<div class="timeline-marker" style="position:absolute; left:${position}%; top:0; width:4px; height:30px; background-color:${color};" 
                    title="${title} - ${eventTime.toLocaleTimeString()}"></div>`;
            });
            
            html += '</div>';
            
            // Add event summary
            html += '<div class="timeline-summary small">';
            html += `<p>Analysis started at ${startTime.toLocaleTimeString()}</p>`;
            
            // Add last event info
            const lastEvent = analysisEvents[analysisEvents.length - 1];
            if (lastEvent.data.type === 'analysis_completed') {
                html += `<p>Completed at ${lastEvent.timestamp.toLocaleTimeString()} (Duration: ${formatDuration(lastEvent.timestamp - startTime)})</p>`;
            } else {
                html += `<p>Last event: ${lastEvent.data.type || 'update'} at ${lastEvent.timestamp.toLocaleTimeString()} (Running: ${formatDuration(currentTime - startTime)})</p>`;
            }
            
            html += `<p>Event count: ${analysisEvents.length}</p>`;
            html += '</div>';
            
            container.innerHTML = html;
        }
        
        // Format duration in ms to readable string
        function formatDuration(ms) {
            if (ms < 1000) return `${ms}ms`;
            
            const seconds = Math.floor(ms / 1000);
            if (seconds < 60) return `${seconds}s`;
            
            const minutes = Math.floor(seconds / 60);
            const remainingSeconds = seconds % 60;
            return `${minutes}m ${remainingSeconds}s`;
        }
        
        // Log message to the message log
        function logMessage(message, type) {
            const logElement = document.getElementById('messageLog');
            const timestamp = new Date().toLocaleTimeString();
            
            let className = '';
            switch(type) {
                case 'in': className = 'message-in'; break;
                case 'out': className = 'message-out'; break;
                case 'error': className = 'message-error'; break;
            }
            
            logElement.innerHTML += `<div class="${className}">[${timestamp}] ${message}</div>`;
            logElement.scrollTop = logElement.scrollHeight;
        }
        
        // Clear logs
        function clearLogs() {
            document.getElementById('messageLog').innerHTML = '';
        }
        
        // Update latency indicator
        function updateLatencyIndicator() {
            if (!lastMessageTime) return;
            
            const now = new Date();
            const latency = now - lastMessageTime;
            
            const indicator = document.getElementById('latencyIndicator');
            
            let color = 'bg-success';
            if (latency > 1000) color = 'bg-danger';
            else if (latency > 300) color = 'bg-warning';
            
            indicator.className = `latency-badge ${color}`;
            indicator.textContent = `${latency}ms`;
        }
        
        // Increment message counter
        function incrementMessageCounter() {
            messageCount++;
            document.getElementById('messageCounter').textContent = `${messageCount} messages`;
        }
        
        // Enable control buttons
        function enableControlButtons() {
            document.getElementById('pauseBtn').disabled = false;
            document.getElementById('resumeBtn').disabled = false;
            document.getElementById('cancelBtn').disabled = false;
            document.getElementById('updateParamsBtn').disabled = false;
        }
        
        // Disable control buttons
        function disableControlButtons() {
            document.getElementById('pauseBtn').disabled = true;
            document.getElementById('resumeBtn').disabled = true;
            document.getElementById('cancelBtn').disabled = true;
            document.getElementById('updateParamsBtn').disabled = true;
        }
        
        // Send pause command
        function sendPauseCommand() {
            if (!socket || socket.readyState !== WebSocket.OPEN) return;
            
            const message = {
                type: 'control_pause',
                timestamp: new Date().toISOString()
            };
            
            socket.send(JSON.stringify(message));
            logMessage(`Sent: ${JSON.stringify(message)}`, 'out');
        }
        
        // Send resume command
        function sendResumeCommand() {
            if (!socket || socket.readyState !== WebSocket.OPEN) return;
            
            const message = {
                type: 'control_resume',
                timestamp: new Date().toISOString()
            };
            
            socket.send(JSON.stringify(message));
            logMessage(`Sent: ${JSON.stringify(message)}`, 'out');
        }
        
        // Send cancel command
        function sendCancelCommand() {
            if (!socket || socket.readyState !== WebSocket.OPEN) return;
            
            const message = {
                type: 'control_cancel',
                timestamp: new Date().toISOString()
            };
            
            socket.send(JSON.stringify(message));
            logMessage(`Sent: ${JSON.stringify(message)}`, 'out');
        }
        
        // Send update params command
        function sendUpdateParamsCommand() {
            if (!socket || socket.readyState !== WebSocket.OPEN) return;
            
            const paramName = document.getElementById('paramName').value.trim();
            const paramValue = document.getElementById('paramValue').value.trim();
            
            if (!paramName || !paramValue) {
                alert('Please enter parameter name and value');
                return;
            }
            
            const params = {};
            params[paramName] = paramValue;
            
            const message = {
                type: 'control_update_params',
                timestamp: new Date().toISOString(),
                meta: params
            };
            
            socket.send(JSON.stringify(message));
            logMessage(`Sent: ${JSON.stringify(message)}`, 'out');
        }
        
        // Send pong message
        function sendPongMessage() {
            if (!socket || socket.readyState !== WebSocket.OPEN) return;
            
            const message = {
                type: 'pong',
                timestamp: new Date().toISOString()
            };
            
            socket.send(JSON.stringify(message));
        }
    </script>
</body>
</html>
EOL

# Make the script executable
chmod +x static/websocket-test.html

echo "WebSocket testing interface has been set up successfully!"
echo "Please add the following route to your application:"
echo "app.Get('/ws-test', websocketTestHandler.ServePage)"
echo ""
echo "Testing interface will be available at: http://localhost:8080/ws-test"