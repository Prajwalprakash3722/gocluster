// Operator Forms JavaScript - Dynamic Form Generator

class OperatorFormsManager {
    constructor() {
        this.currentOperator = null;
        this.currentOperation = null;
        this.currentSchema = null;
        this.formData = {};
        this.operators = {};
        this.init();
    }

    init() {
        console.log('Initializing OperatorFormsManager...');
        
        // Initialize with template data if available
        if (window.templateOperators) {
            console.log('Found template operators:', Object.keys(window.templateOperators));
            this.operators = window.templateOperators;
            
            // Add a small delay to ensure DOM is fully rendered
            setTimeout(() => {
                this.renderOperatorsFromTemplate();
            }, 100);
        } else {
            console.log('No template operators found, loading from API...');
            this.loadOperators();
        }
        this.setupEventListeners();
        
        // Initialize form manager
        window.operatorForms = this;
        console.log('OperatorFormsManager initialized');
    }

    setupEventListeners() {
        // Keyboard shortcuts
        document.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
                this.executeCurrentOperation();
            }
            if (e.key === 'Escape') {
                this.hideJSONMode();
            }
        });
    }

    renderOperatorsFromTemplate() {
        // If operators are already rendered in template, just bind events
        const operatorCards = document.querySelectorAll('.operator-card');
        console.log('Found operator cards:', operatorCards.length);
        
        operatorCards.forEach(card => {
            card.addEventListener('click', (event) => {
                event.preventDefault();
                event.stopPropagation();
                
                const operatorName = card.getAttribute('data-operator');
                console.log('Clicked operator:', operatorName);
                
                if (operatorName && this.operators[operatorName]) {
                    this.selectOperator(operatorName, this.operators[operatorName], event);
                } else {
                    console.error('Operator not found:', operatorName, 'Available:', Object.keys(this.operators));
                }
            });
            
            // Add hover effects
            card.addEventListener('mouseenter', () => {
                if (!card.classList.contains('ring-2')) {
                    card.classList.add('bg-gray-700');
                }
            });
            
            card.addEventListener('mouseleave', () => {
                card.classList.remove('bg-gray-700');
            });
        });
    }

    async loadOperators() {
        try {
            const response = await fetch('/api/operators');
            const data = await response.json();
            
            if (data.success) {
                this.operators = data.data;
                this.renderOperators(data.data);
            } else {
                this.showNotification('Failed to load operators: ' + data.error, 'error');
            }
        } catch (error) {
            this.showNotification('Network error loading operators: ' + error.message, 'error');
        }
    }

    renderOperators(operators) {
        const container = document.getElementById('operatorList');
        container.innerHTML = '';

        Object.entries(operators).forEach(([name, info]) => {
            const operatorCard = document.createElement('div');
            operatorCard.className = 'operator-card card p-4 rounded-lg cursor-pointer transition-all';
            operatorCard.onclick = () => this.selectOperator(name, info);
            
            operatorCard.innerHTML = `
                <div class="flex items-center justify-between">
                    <div>
                        <h4 class="font-medium text-white">${info.name}</h4>
                        <p class="text-sm text-gray-400">${info.description}</p>
                        <p class="text-xs text-gray-500 mt-1">v${info.version} by ${info.author}</p>
                    </div>
                    <i class="fas fa-chevron-right text-gray-400"></i>
                </div>
            `;
            
            container.appendChild(operatorCard);
        });
    }

    async selectOperator(name, info, event = null) {
        console.log('Selecting operator:', name);
        this.currentOperator = name;
        
        // Update UI to show selection
        document.querySelectorAll('.operator-card').forEach(card => {
            card.classList.remove('ring-2', 'ring-blue-500', 'bg-blue-900');
            card.classList.add('bg-gray-800');
        });
        
        // Add selection styling to clicked card
        let selectedCard = null;
        if (event && event.currentTarget) {
            selectedCard = event.currentTarget;
        } else {
            // Fallback: find the card by data-operator attribute
            selectedCard = document.querySelector(`[data-operator="${name}"]`);
        }
        
        if (selectedCard) {
            selectedCard.classList.remove('bg-gray-800');
            selectedCard.classList.add('ring-2', 'ring-blue-500', 'bg-blue-900');
        }

        // Load operator schema
        try {
            const response = await fetch(`/api/operator/schema/${name}`);
            const data = await response.json();
            
            if (data.success) {
                this.currentSchema = data.data;
                this.loadOperations();
                console.log('Loaded schema for operator:', name);
            } else {
                this.showNotification('Failed to load operator schema: ' + data.error, 'error');
            }
        } catch (error) {
            this.showNotification('Network error loading schema: ' + error.message, 'error');
            console.error('Error loading schema:', error);
        }
    }

    loadOperations() {
        const header = document.getElementById('operationHeader');
        const select = document.getElementById('operationSelect');
        const title = document.getElementById('operationTitle');
        
        header.style.display = 'block';
        title.textContent = `Execute ${this.currentSchema.name} Operations`;
        
        // Populate operations dropdown
        select.innerHTML = '<option value="">Select an operation...</option>';
        
        Object.entries(this.currentSchema.operations).forEach(([operation, schema]) => {
            const option = document.createElement('option');
            option.value = operation;
            option.textContent = `${operation} - ${schema.description}`;
            select.appendChild(option);
        });

        // Clear form container
        this.clearFormContainer();
    }

    loadOperationForm() {
        const select = document.getElementById('operationSelect');
        const operation = select.value;
        
        if (!operation) {
            this.clearFormContainer();
            return;
        }

        this.currentOperation = operation;
        const operationSchema = this.currentSchema.operations[operation];
        
        // Update description
        document.getElementById('operationDescription').textContent = operationSchema.description;
        
        // Generate form
        this.generateForm(operationSchema);
    }

    generateForm(operationSchema) {
        const container = document.getElementById('formContainer');
        const { parameters, examples } = operationSchema;
        
        let formHTML = '<form id="operationForm" onsubmit="operatorForms.executeOperation(event)">';
        
        if (Object.keys(parameters).length === 0) {
            formHTML += `
                <div class="text-center text-gray-400 py-8">
                    <i class="fas fa-info-circle text-2xl mb-2"></i>
                    <p>This operation requires no parameters</p>
                </div>
            `;
        } else {
            formHTML += '<div class="space-y-4">';
            
            Object.entries(parameters).forEach(([paramName, paramSchema]) => {
                formHTML += this.generateFormField(paramName, paramSchema);
            });
            
            formHTML += '</div>';
        }
        
        // Add examples section
        if (examples && examples.length > 0) {
            formHTML += this.generateExamplesSection(examples);
        }
        
        // Add execution strategy section
        formHTML += this.generateExecutionStrategySection();
        
        // Add action buttons
        formHTML += `
            <div class="flex justify-end space-x-3 mt-6">
                <button type="button" onclick="operatorForms.clearForm()" class="btn-secondary px-4 py-2 rounded-md text-sm font-medium">
                    <i class="fas fa-eraser mr-2"></i>Clear
                </button>
                <button type="submit" class="btn-primary px-4 py-2 rounded-md text-sm font-medium">
                    <i class="fas fa-play mr-2"></i>Execute
                </button>
            </div>
        `;
        
        formHTML += '</form>';
        
        container.innerHTML = formHTML;
    }

    generateFormField(paramName, paramSchema) {
        const { type, required, description, example, options, default: defaultValue } = paramSchema;
        const fieldId = `param_${paramName}`;
        const requiredClass = required ? 'required-field' : '';
        
        let fieldHTML = `
            <div class="form-field">
                <label for="${fieldId}" class="block text-sm font-medium text-gray-300 mb-2 ${requiredClass}">
                    ${this.formatFieldName(paramName)}
                </label>
        `;
        
        if (description) {
            fieldHTML += `<p class="text-xs text-gray-400 mb-2">${description}</p>`;
        }
        
        switch (type) {
            case 'string':
                if (options && options.length > 0) {
                    // Select dropdown for enum-like fields
                    fieldHTML += `<select id="${fieldId}" name="${paramName}" class="form-input w-full px-3 py-2 rounded-md" ${required ? 'required' : ''}>`;
                    fieldHTML += `<option value="">Select ${paramName}...</option>`;
                    options.forEach(option => {
                        const selected = option === defaultValue ? 'selected' : '';
                        fieldHTML += `<option value="${option}" ${selected}>${option}</option>`;
                    });
                    fieldHTML += '</select>';
                } else {
                    // Regular text input
                    const placeholder = example ? `e.g., ${example}` : '';
                    const value = defaultValue || '';
                    fieldHTML += `<input type="text" id="${fieldId}" name="${paramName}" class="form-input w-full px-3 py-2 rounded-md" placeholder="${placeholder}" value="${value}" ${required ? 'required' : ''}>`;
                }
                break;
                
            case 'int':
            case 'float':
                const placeholder = example ? `e.g., ${example}` : '';
                const value = defaultValue || '';
                const step = type === 'float' ? 'step="any"' : '';
                fieldHTML += `<input type="number" id="${fieldId}" name="${paramName}" class="form-input w-full px-3 py-2 rounded-md" placeholder="${placeholder}" value="${value}" ${step} ${required ? 'required' : ''}>`;
                break;
                
            case 'bool':
                const checked = defaultValue ? 'checked' : '';
                fieldHTML += `
                    <div class="flex items-center">
                        <input type="checkbox" id="${fieldId}" name="${paramName}" class="form-input mr-3" ${checked}>
                        <label for="${fieldId}" class="text-sm text-gray-300">Enable ${this.formatFieldName(paramName)}</label>
                    </div>
                `;
                break;
                
            case 'object':
                const objPlaceholder = example ? JSON.stringify(example, null, 2) : '{}';
                fieldHTML += `<textarea id="${fieldId}" name="${paramName}" class="form-input w-full px-3 py-2 rounded-md font-mono text-sm" rows="6" placeholder="${objPlaceholder}" ${required ? 'required' : ''}></textarea>`;
                break;
                
            case 'array':
                const arrayPlaceholder = example ? JSON.stringify(example, null, 2) : '[]';
                fieldHTML += `<textarea id="${fieldId}" name="${paramName}" class="form-input w-full px-3 py-2 rounded-md font-mono text-sm" rows="4" placeholder="${arrayPlaceholder}" ${required ? 'required' : ''}></textarea>`;
                break;
                
            default:
                // Fallback to text input
                const defaultPlaceholder = example ? `e.g., ${example}` : '';
                fieldHTML += `<input type="text" id="${fieldId}" name="${paramName}" class="form-input w-full px-3 py-2 rounded-md" placeholder="${defaultPlaceholder}" ${required ? 'required' : ''}>`;
        }
        
        fieldHTML += '</div>';
        return fieldHTML;
    }

    generateExamplesSection(examples) {
        let html = `
            <div class="mt-6 p-4 bg-gh-dark rounded-lg border border-gh-border">
                <h4 class="text-sm font-medium text-gray-300 mb-3">
                    <i class="fas fa-lightbulb text-yellow-400 mr-2"></i>
                    Examples
                </h4>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
        `;
        
        examples.forEach((example, index) => {
            html += `
                <button type="button" onclick="operatorForms.loadExample(${index})" class="example-btn px-3 py-2 rounded-md text-white text-left">
                    <div class="text-xs font-medium mb-1">Example ${index + 1}</div>
                    <div class="text-xs opacity-75 font-mono">${this.summarizeExample(example)}</div>
                </button>
            `;
        });
        
        html += '</div></div>';
        return html;
    }

    summarizeExample(example) {
        const summary = Object.entries(example).slice(0, 2).map(([key, value]) => {
            const displayValue = typeof value === 'string' ? value.substring(0, 15) : JSON.stringify(value).substring(0, 15);
            return `${key}: ${displayValue}`;
        }).join(', ');
        
        return Object.keys(example).length > 2 ? summary + '...' : summary;
    }

    loadExample(exampleIndex) {
        const examples = this.currentSchema.operations[this.currentOperation].examples;
        const example = examples[exampleIndex];
        
        Object.entries(example).forEach(([paramName, value]) => {
            const field = document.getElementById(`param_${paramName}`);
            if (field) {
                if (field.type === 'checkbox') {
                    field.checked = Boolean(value);
                } else if (typeof value === 'object') {
                    field.value = JSON.stringify(value, null, 2);
                } else {
                    field.value = value;
                }
            }
        });
        
        this.showNotification('Example loaded successfully', 'info');
    }

    async executeOperation(event) {
        if (event) event.preventDefault();
        
        if (!this.currentOperator || !this.currentOperation) {
            this.showNotification('Please select an operator and operation', 'error');
            return;
        }
        
        // Collect form data
        const formData = new FormData(document.getElementById('operationForm'));
        const params = {};
        
        for (const [key, value] of formData.entries()) {
            if (key === 'executionType' || key === 'targetNode' || key === 'rollingDelay' || 
                key === 'failFast' || key === 'continueOnError' || key === 'executionTimeout') {
                continue; // Skip execution strategy fields
            }
            
            const field = document.getElementById(`param_${key}`);
            
            if (field && field.type === 'checkbox') {
                params[key] = field.checked;
            } else if (field && field.name.endsWith('[]')) {
                // Handle array fields
                try {
                    params[key] = JSON.parse(value || '[]');
                } catch (e) {
                    this.showNotification(`Invalid JSON in ${key}: ${e.message}`, 'error');
                    return;
                }
            } else if (value && (value.trim().startsWith('{') || value.trim().startsWith('['))) {
                // Handle object/array fields
                try {
                    params[key] = JSON.parse(value);
                } catch (e) {
                    this.showNotification(`Invalid JSON in ${key}: ${e.message}`, 'error');
                    return;
                }
            } else if (field && field.type === 'number') {
                params[key] = field.step === 'any' ? parseFloat(value) : parseInt(value);
            } else if (value) {
                params[key] = value;
            }
        }

        // Get execution strategy
        const executionType = document.getElementById('executionType').value;
        const timeout = document.getElementById('executionTimeout').value;
        
        // Show loading state
        const submitBtn = document.querySelector('button[type="submit"]');
        const originalText = submitBtn.innerHTML;
        submitBtn.innerHTML = '<div class="loading-spinner inline-block mr-2"></div>Executing...';
        submitBtn.disabled = true;
        
        try {
            let endpoint = `/api/operator/trigger/${this.currentOperator}`;
            let requestBody = {
                operation: this.currentOperation,
                params: params
            };

            // Handle different execution strategies
            switch (executionType) {
                case 'single':
                    const targetNode = document.getElementById('targetNode').value;
                    if (!targetNode) {
                        this.showNotification('Please select a target node', 'error');
                        return;
                    }
                    requestBody.node_id = targetNode;
                    break;

                case 'broadcast':
                    endpoint = `/api/operator/broadcast/${this.currentOperator}`;
                    const failFast = document.getElementById('failFast').checked;
                    const continueOnError = document.getElementById('continueOnError').checked;
                    requestBody.fail_fast = failFast;
                    requestBody.continue_on_error = continueOnError;
                    break;

                case 'rolling':
                    endpoint = `/api/operator/rolling/${this.currentOperator}`;
                    const delay = document.getElementById('rollingDelay').value;
                    const nodeOrder = this.getNodeExecutionOrder();
                    requestBody.node_order = nodeOrder;
                    requestBody.delay = delay;
                    break;

                case 'local':
                default:
                    // Default local execution - no additional parameters needed
                    break;
            }
            
            const url = timeout ? `${endpoint}?timeout=${timeout}` : endpoint;
            
            const response = await fetch(url, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(requestBody)
            });
            
            const result = await response.json();
            this.displayResult(result);
            
            if (result.success) {
                this.showNotification(`Operation executed successfully! ${this.getExecutionMessage(executionType, result)}`, 'success');
            } else {
                this.showNotification('Operation failed: ' + result.error, 'error');
            }
            
        } catch (error) {
            this.showNotification('Network error: ' + error.message, 'error');
        } finally {
            // Restore button state
            submitBtn.innerHTML = originalText;
            submitBtn.disabled = false;
        }
    }

    getExecutionMessage(executionType, result) {
        switch (executionType) {
            case 'single':
                return `Executed on node ${result.data?.node_id || 'unknown'}`;
            case 'broadcast':
                return `Broadcasted to all nodes`;
            case 'rolling':
                return `Rolling execution started`;
            default:
                return 'Executed locally';
        }
    }

    getNodeExecutionOrder() {
        const orderItems = document.querySelectorAll('.node-order-item');
        return Array.from(orderItems).map(item => item.getAttribute('data-node-id'));
    }

    formatFieldName(name) {
        return name.split('_').map(word => 
            word.charAt(0).toUpperCase() + word.slice(1)
        ).join(' ');
    }

    displayResult(result) {
        const container = document.getElementById('resultContainer');
        const content = document.getElementById('resultContent');
        
        content.textContent = JSON.stringify(result, null, 2);
        container.style.display = 'block';
        
        // Scroll to result
        container.scrollIntoView({ behavior: 'smooth' });
    }

    clearForm() {
        const form = document.getElementById('operationForm');
        if (form) {
            form.reset();
        }
        
        const resultContainer = document.getElementById('resultContainer');
        resultContainer.style.display = 'none';
    }

    clearFormContainer() {
        const container = document.getElementById('formContainer');
        container.innerHTML = `
            <div class="text-center text-gray-400 py-12">
                <i class="fas fa-mouse-pointer text-4xl mb-4"></i>
                <p>Select an operation to begin</p>
            </div>
        `;
    }

    generateExecutionStrategySection() {
        const nodes = window.clusterData?.nodes || {};
        const nodeOptions = Object.entries(nodes).map(([nodeId, node]) => {
            const leaderLabel = node.isLeader ? ' (LEADER)' : '';
            const statusIcon = node.status === 'running' ? '🟢' : '🔴';
            return `<option value="${nodeId}">${statusIcon} ${nodeId}${leaderLabel}</option>`;
        }).join('');

        return `
            <div class="mt-6 p-4 bg-gray-800 rounded-lg border border-gray-700">
                <h4 class="text-sm font-medium text-gray-300 mb-4">
                    <i class="fas fa-network-wired text-blue-400 mr-2"></i>
                    Execution Strategy
                </h4>
                
                <div class="space-y-4">
                    <!-- Execution Type -->
                    <div>
                        <label class="block text-sm font-medium text-gray-300 mb-2">Execution Type</label>
                        <select id="executionType" name="executionType" class="form-input w-full px-3 py-2 rounded-md" onchange="operatorForms.toggleExecutionOptions()">
                            <option value="local">Local Only (Current Node)</option>
                            <option value="single">Single Node</option>
                            <option value="broadcast">All Nodes (Parallel)</option>
                            <option value="rolling">Rolling Execution (Sequential)</option>
                        </select>
                    </div>

                    <!-- Single Node Selection -->
                    <div id="singleNodeSection" class="hidden">
                        <label for="targetNode" class="block text-sm font-medium text-gray-300 mb-2">Target Node</label>
                        <select id="targetNode" name="targetNode" class="form-input w-full px-3 py-2 rounded-md">
                            <option value="">Select target node...</option>
                            ${nodeOptions}
                        </select>
                    </div>

                    <!-- Rolling Execution Options -->
                    <div id="rollingSection" class="hidden space-y-3">
                        <div>
                            <label for="rollingDelay" class="block text-sm font-medium text-gray-300 mb-2">Delay Between Nodes</label>
                            <select id="rollingDelay" name="rollingDelay" class="form-input w-full px-3 py-2 rounded-md">
                                <option value="0s">No Delay</option>
                                <option value="5s">5 seconds</option>
                                <option value="10s">10 seconds</option>
                                <option value="30s" selected>30 seconds</option>
                                <option value="1m">1 minute</option>
                                <option value="2m">2 minutes</option>
                                <option value="5m">5 minutes</option>
                            </select>
                        </div>
                        
                        <div>
                            <label for="nodeOrder" class="block text-sm font-medium text-gray-300 mb-2">
                                Execution Order
                                <span class="text-xs text-gray-400">(drag to reorder)</span>
                            </label>
                            <div id="nodeOrderList" class="space-y-2 max-h-40 overflow-y-auto border border-gray-600 rounded-md p-3">
                                ${this.generateNodeOrderList(nodes)}
                            </div>
                        </div>
                    </div>

                    <!-- Broadcast Options -->
                    <div id="broadcastSection" class="hidden">
                        <div class="flex items-center space-x-3">
                            <input type="checkbox" id="failFast" name="failFast" class="rounded border-gray-600 bg-gray-700 text-blue-400 focus:ring-blue-400">
                            <label for="failFast" class="text-sm text-gray-300">Stop on first failure</label>
                        </div>
                        <div class="flex items-center space-x-3 mt-2">
                            <input type="checkbox" id="continueOnError" name="continueOnError" class="rounded border-gray-600 bg-gray-700 text-blue-400 focus:ring-blue-400" checked>
                            <label for="continueOnError" class="text-sm text-gray-300">Continue on errors</label>
                        </div>
                    </div>

                    <!-- Timeout Setting -->
                    <div>
                        <label for="executionTimeout" class="block text-sm font-medium text-gray-300 mb-2">Execution Timeout</label>
                        <select id="executionTimeout" name="executionTimeout" class="form-input w-full px-3 py-2 rounded-md">
                            <option value="30s" selected>30 seconds</option>
                            <option value="1m">1 minute</option>
                            <option value="2m">2 minutes</option>
                            <option value="5m">5 minutes</option>
                            <option value="10m">10 minutes</option>
                            <option value="30m">30 minutes</option>
                        </select>
                    </div>
                </div>
            </div>
        `;
    }

    generateNodeOrderList(nodes) {
        return Object.entries(nodes).map(([nodeId, node], index) => {
            const leaderLabel = node.isLeader ? ' (LEADER)' : '';
            const statusIcon = node.status === 'running' ? '🟢' : '🔴';
            return `
                <div class="node-order-item flex items-center justify-between p-2 bg-gray-700 rounded border border-gray-600 cursor-move" data-node-id="${nodeId}">
                    <div class="flex items-center space-x-2">
                        <i class="fas fa-grip-vertical text-gray-400"></i>
                        <span class="text-sm font-medium">${index + 1}.</span>
                        <span>${statusIcon} ${nodeId}${leaderLabel}</span>
                    </div>
                    <div class="text-xs text-gray-400">${node.address || 'Unknown'}</div>
                </div>
            `;
        }).join('');
    }

    toggleExecutionOptions() {
        const executionType = document.getElementById('executionType').value;
        
        // Hide all sections first
        document.getElementById('singleNodeSection').classList.add('hidden');
        document.getElementById('rollingSection').classList.add('hidden');
        document.getElementById('broadcastSection').classList.add('hidden');
        
        // Show relevant section
        switch (executionType) {
            case 'single':
                document.getElementById('singleNodeSection').classList.remove('hidden');
                break;
            case 'rolling':
                document.getElementById('rollingSection').classList.remove('hidden');
                this.initializeNodeSorting();
                break;
            case 'broadcast':
                document.getElementById('broadcastSection').classList.remove('hidden');
                break;
        }
    }

    initializeNodeSorting() {
        // Simple drag and drop for node ordering
        const nodeList = document.getElementById('nodeOrderList');
        let draggedElement = null;

        nodeList.addEventListener('dragstart', (e) => {
            if (e.target.classList.contains('node-order-item')) {
                draggedElement = e.target;
                e.target.style.opacity = '0.5';
            }
        });

        nodeList.addEventListener('dragend', (e) => {
            if (e.target.classList.contains('node-order-item')) {
                e.target.style.opacity = '';
                draggedElement = null;
            }
        });

        nodeList.addEventListener('dragover', (e) => {
            e.preventDefault();
        });

        nodeList.addEventListener('drop', (e) => {
            e.preventDefault();
            if (draggedElement && e.target.classList.contains('node-order-item')) {
                const rect = e.target.getBoundingClientRect();
                const midpoint = rect.top + rect.height / 2;
                
                if (e.clientY < midpoint) {
                    nodeList.insertBefore(draggedElement, e.target);
                } else {
                    nodeList.insertBefore(draggedElement, e.target.nextSibling);
                }
                
                this.updateNodeOrderNumbers();
            }
        });

        // Make items draggable
        nodeList.querySelectorAll('.node-order-item').forEach(item => {
            item.draggable = true;
        });
    }

    updateNodeOrderNumbers() {
        const items = document.querySelectorAll('.node-order-item');
        items.forEach((item, index) => {
            const numberSpan = item.querySelector('span');
            if (numberSpan) {
                numberSpan.textContent = `${index + 1}.`;
            }
        });
    }

    // JSON Mode functions
    showJSONMode() {
        document.getElementById('jsonModal').classList.remove('hidden');
        document.getElementById('jsonModal').classList.add('flex');
    }

    hideJSONMode() {
        document.getElementById('jsonModal').classList.add('hidden');
        document.getElementById('jsonModal').classList.remove('flex');
    }

    async executeJSONRequest() {
        const input = document.getElementById('jsonInput').value;
        const output = document.getElementById('jsonOutput');
        
        if (!input.trim()) {
            this.showNotification('Please enter a JSON request', 'error');
            return;
        }
        
        try {
            const request = JSON.parse(input);
            
            if (!this.currentOperator) {
                this.showNotification('Please select an operator first', 'error');
                return;
            }
            
            const response = await fetch(`/api/operator/trigger/${this.currentOperator}`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: input
            });
            
            const result = await response.json();
            output.textContent = JSON.stringify(result, null, 2);
            
            if (result.success) {
                this.showNotification('JSON request executed successfully', 'success');
            } else {
                this.showNotification('JSON request failed: ' + result.error, 'error');
            }
            
        } catch (error) {
            this.showNotification('Invalid JSON or network error: ' + error.message, 'error');
        }
    }

    showNotification(message, type) {
        const notification = document.createElement('div');
        notification.className = `notification ${type}`;
        notification.innerHTML = `
            <div class="flex items-center">
                <i class="fas fa-${this.getNotificationIcon(type)} mr-3"></i>
                <span>${message}</span>
            </div>
        `;
        
        document.body.appendChild(notification);
        
        // Show notification
        setTimeout(() => notification.classList.add('show'), 100);
        
        // Auto remove after 5 seconds
        setTimeout(() => {
            notification.classList.remove('show');
            setTimeout(() => notification.remove(), 300);
        }, 5000);
    }

    getNotificationIcon(type) {
        switch (type) {
            case 'success': return 'check-circle';
            case 'error': return 'exclamation-circle';
            case 'info': return 'info-circle';
            default: return 'bell';
        }
    }

    executeCurrentOperation() {
        const form = document.getElementById('operationForm');
        if (form) {
            this.executeOperation();
        }
    }
}

// Global functions for HTML onclick handlers
function refreshOperators() {
    operatorForms.loadOperators();
}

function loadOperationForm() {
    operatorForms.loadOperationForm();
}

function clearForm() {
    operatorForms.clearForm();
}

function showJSONMode() {
    operatorForms.showJSONMode();
}

function hideJSONMode() {
    operatorForms.hideJSONMode();
}

function executeJSONRequest() {
    operatorForms.executeJSONRequest();
}

// Initialize when page loads
document.addEventListener('DOMContentLoaded', () => {
    window.operatorForms = new OperatorFormsManager();
});
