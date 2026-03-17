import React from "react";
import * as monaco from "monaco-editor";
import { message, Collapse } from "@kweaver-ai/ui";
import Form, { Templates } from "@rjsf/mui";
import VisibilityIcon from "@mui/icons-material/Visibility";
import VisibilityOffIcon from "@mui/icons-material/VisibilityOff";
import InputAdornment from "@mui/material/InputAdornment";
import IconButton from "@mui/material/IconButton";
// 不这样引入import * as monaco from "monaco-editor";
// 是为了防止报错unexpected usage
// import * as monaco from "monaco-editor/esm/vs/editor/editor.api";
import {
  RJSFSchema,
  UiSchema,
  BaseInputTemplateProps,
  WidgetProps,
} from "@rjsf/utils";
import { ServiceFrameworkBase } from "./component.base";
import Editor, { loader } from "@monaco-editor/react";
import validator from "@rjsf/validator-ajv8";
import styles from "./styles.module.less";
import { isEqualWith } from "lodash";
import __ from "./locale";
import TextField from "@mui/material/TextField";
import MenuItem from "@mui/material/MenuItem";

const log = (type: string) => console.log.bind(console, type);

// 防止代码变成一行
const formatString = (o: Object) => JSON.stringify(o, null, 2);

// 预加载编辑器，防止一直loading
loader.config({ monaco });

// 从mui中导出基础组件
const { BaseInputTemplate } = Templates as {
  BaseInputTemplate: React.ElementType;
};

// 自定义修改基础组件属性
function MyBaseInputTemplate(props: BaseInputTemplateProps) {
  const [showPassword, setShowPassword] = React.useState<boolean>(false);
  const handleClickShowPassword = () => setShowPassword((show) => !show);
  // 属性定义可参考：https://mui.com/material-ui/api/text-field/
  let customProps = {};
  if (props.type === "password") {
    // 方式1：可通过修改属性实现，可参考：https://stackoverflow.com/questions/60391113/how-to-view-password-from-material-ui-textfield
    // 方式2：劫持整个密码输入框组件，自定义该组件
    customProps = {
      ...customProps,
      // 密码框的小眼睛
      type: showPassword ? "string" : "password",
      InputProps: {
        endAdornment: (
          <InputAdornment position="end">
            <IconButton onClick={handleClickShowPassword}>
              {showPassword ? <VisibilityOffIcon /> : <VisibilityIcon />}
            </IconButton>
          </InputAdornment>
        ),
      },
    };
  } else if (
    props.schema.type === "string" &&
    props.schema.title === "password"
  ) {
    // 该判断无用
    // 劫持原始数字输入框类型
    customProps = {
      ...customProps,
      // type: "password",
    };
  } else if (
    props.schema.type === "integer" ||
    props.schema.type === "number"
  ) {
    // 劫持原始数字输入框类型
    customProps = {
      ...customProps,
      // type可以是：https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input#Form_%3Cinput%3E_types
      type: "string",
    };
  }
  // 自定义修改基础组件属性
  return <BaseInputTemplate {...props} {...customProps} />;
}

// 自定义boolean下拉选择组件：true/false/undefined（undefined时移除该字段）
function BooleanSelectWidget(props: WidgetProps) {
  const {
    id,
    value,
    disabled,
    readonly,
    autofocus,
    onChange,
    rawErrors,
    label,
    schema,
  } = props;

  const currentValue =
    value === true ? "true" : value === false ? "false" : "undefined";

  const handleChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const selected = event.target.value;
    if (selected === "true") {
      onChange(true);
    } else if (selected === "false") {
      onChange(false);
    } else {
      // 选择undefined时，移除该字段
      onChange(undefined);
    }
  };

  return (
    <TextField
      id={id}
      select
      fullWidth
      label={label}
      helperText={schema?.description}
      value={currentValue}
      onChange={handleChange}
      disabled={disabled || readonly}
      autoFocus={autofocus}
      error={Array.isArray(rawErrors) && rawErrors.length > 0}
    >
      <MenuItem value="undefined">{__("未设置")}</MenuItem>
      <MenuItem value="true">true</MenuItem>
      <MenuItem value="false">false</MenuItem>
    </TextField>
  );
}

export class ServiceFramework extends ServiceFrameworkBase {
  render() {
    const { isReadOnly } = this.props;

    const formData = this.props.formData || {};
    const SCHEMA = this.props.schema || {};
    const UISchema = this.props.uiSchema || {};
    const onChangeFormData = this.props.onChangeFormData || (() => {});
    const changeIsFormValidator =
      this.props.changeIsFormValidator || (() => {});
    // 编辑器配置
    const monacoEditorOptions = {
      automaticLayout: true,
      readOnly: isReadOnly,
    };
    // uiSchema
    const uiSchema: UiSchema = {
      "ui:submitButtonOptions": this.props.setCallback
        ? {
            norender: true,
          }
        : {
            props: {
              className: "ai-btn-primary",
              style: {
                fontSize: "14px",
                padding: "4px 15px",
              },
            },
            norender: isReadOnly || Object.keys(SCHEMA).length === 0,
            submitText: __("提 交"),
          },
      ...UISchema,
    };

    return (
      <div className={styles["service-framework"]}>
        <div className={styles.left}>
          <Collapse defaultActiveKey={[]} className="service-collapse" ghost>
            <Collapse.Panel header={__("配置项数据")} key="1">
              <Editor
                language="json"
                value={formatString(formData)}
                theme="vs-light"
                onChange={(newFormData: any) => {
                  if (
                    !isEqualWith(
                      newFormData,
                      formData,
                      (newValue: string, oldValue: string) => {
                        // Since this is coming from the editor which uses JSON.stringify to trim undefined values compare the values
                        // using JSON.stringify to see if the trimmed formData is the same as the untrimmed state
                        // Sometimes passing the trimmed value back into the Form causes the defaults to be improperly assigned
                        return (
                          JSON.stringify(oldValue) === JSON.stringify(newValue)
                        );
                      }
                    )
                  ) {
                    onChangeFormData!(JSON.parse(newFormData));
                    changeIsFormValidator(false);
                  }
                }}
                options={monacoEditorOptions}
              />
            </Collapse.Panel>
          </Collapse>
        </div>
        <div className={styles.right}>
          <Form
            ref={this.formRef}
            schema={SCHEMA as RJSFSchema}
            experimental_defaultFormStateBehavior={{
              emptyObjectFields: "skipEmptyDefaults",
            }}
            templates={{ BaseInputTemplate: MyBaseInputTemplate }}
            widgets={{ CheckboxWidget: BooleanSelectWidget }}
            uiSchema={uiSchema}
            validator={validator}
            formData={formData}
            onChange={({ formData }) => {
              onChangeFormData!(formData);
              changeIsFormValidator(false);
            }}
            onSubmit={() => {
              changeIsFormValidator(true);
              message.success(__("配置项验证通过"));
            }}
            onError={() => changeIsFormValidator(false)}
            readonly={isReadOnly}
            showErrorList={false}
            focusOnFirstError
          />
        </div>
      </div>
    );
  }
}
